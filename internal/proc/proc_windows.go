//go:build windows

// Package proc — Windows implementation.
//
// Design: we use a single Toolhelp32 snapshot for enumeration (phase 1) and
// PEB reads via ReadProcessMemory for env fetch (phase 2). Both paths run
// without CGO and without any third-party tools.
//
// Scope and limits for v1 (documented in the design doc):
//
//   - 64-bit scanner reading 64-bit target processes: full support (start
//     time, user, command line, env).
//   - 32-bit target processes on a 64-bit host (WOW64): env read is skipped
//     and a `wow64_unsupported` warning is emitted. The 32-bit PEB lives at
//     a different address with different struct offsets; correctly reading
//     it requires NtWow64QueryInformationProcess64 / ReadVirtualMemory64.
//     That lands in a follow-up commit.
//   - Protected processes (antivirus, DRM, lsass, csrss): OpenProcess fails
//     with ERROR_ACCESS_DENIED. We emit `permission_denied` and keep going.
//     Run the scanner elevated to see more of them.
//
// PEB offsets below are for x64 Windows 10+. They match winternl.h and are
// stable. If Microsoft ever moves them, the scan still succeeds for every
// same-bitness read that doesn't use the moved field — FetchEnv emits a
// warning, List still reports the process with name + pid.
package proc

import (
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// 64-bit PEB layout. See winternl.h and Windows Internals 7e.
const (
	pebOffsetProcessParameters = 0x20  // PEB -> RTL_USER_PROCESS_PARAMETERS*
	rupOffsetCommandLine       = 0x70  // UNICODE_STRING {Length, MaxLen, Buffer}
	rupOffsetEnvironment       = 0x80  // PVOID
	rupOffsetEnvironmentSize   = 0x3F0 // SIZE_T (Win10+; older Windows puts it elsewhere)

	unicodeStringSize = 0x10 // Length (USHORT) + MaxLen (USHORT) + pad + Buffer (PWSTR)
)

// processBasicInformation mirrors PROCESS_BASIC_INFORMATION from winternl.h.
// Only PebBaseAddress is used here.
type processBasicInformation struct {
	ExitStatus                   uintptr
	PebBaseAddress               uintptr
	AffinityMask                 uintptr
	BasePriority                 uintptr
	UniqueProcessId              uintptr
	InheritedFromUniqueProcessId uintptr
}

// NewLister returns the Windows process lister.
func NewLister() Lister {
	return &windowsLister{}
}

type windowsLister struct{}

// List enumerates processes via Toolhelp32, then for each process opens a
// handle and enriches with start time, user, and command line. Failures
// during enrichment are non-fatal — the process still appears in the output
// with at least PID and Name.
func (l *windowsLister) List() ([]Process, []Warning, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("CreateToolhelp32Snapshot: %w", err)
	}
	defer windows.CloseHandle(snap)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err := windows.Process32First(snap, &entry); err != nil {
		return nil, nil, fmt.Errorf("Process32First: %w", err)
	}

	var procs []Process
	var warnings []Warning
	for {
		pid := int(entry.ProcessID)
		name := windows.UTF16ToString(entry.ExeFile[:])

		// Skip System Idle Process (pid 0) and the System process (pid 4).
		// They are not meaningful scan targets and OpenProcess will fail
		// on them regardless.
		if pid == 0 || pid == 4 {
			if err := windows.Process32Next(snap, &entry); err != nil {
				break
			}
			continue
		}

		p := Process{PID: pid, Name: name, Cmd: name}
		enrichProcess(pid, &p)
		procs = append(procs, p)

		if err := windows.Process32Next(snap, &entry); err != nil {
			break
		}
	}

	return procs, warnings, nil
}

// enrichProcess opens the process and populates StartTime, User, and Cmd.
// All enrichment is best-effort — an inaccessible process still ships with
// the pid and name we got from Toolhelp32.
func enrichProcess(pid int, p *Process) {
	// Try with VM_READ for command-line extraction. Fall back to the cheaper
	// rights set if that's refused — start time and user don't need VM_READ.
	h, err := windows.OpenProcess(
		windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_VM_READ,
		false, uint32(pid))
	if err != nil {
		h2, err2 := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
		if err2 != nil {
			return
		}
		defer windows.CloseHandle(h2)
		fillTimeAndUser(h2, p)
		return
	}
	defer windows.CloseHandle(h)

	fillTimeAndUser(h, p)
	if cl, ok := readCommandLine(h); ok && cl != "" {
		p.Cmd = cl
	}
}

func fillTimeAndUser(h windows.Handle, p *Process) {
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &creation, &exit, &kernel, &user); err == nil {
		// Filetime.Nanoseconds() returns nanoseconds since the Unix epoch.
		p.StartTime = time.Unix(0, creation.Nanoseconds())
	}
	if u, ok := processUser(h); ok {
		p.User = u
	}
}

// processUser reads the token and resolves it to a "DOMAIN\user" string.
func processUser(h windows.Handle) (string, bool) {
	const tokenQuery = 0x0008 // TOKEN_QUERY; not re-exported by x/sys but stable
	var tok windows.Token
	if err := windows.OpenProcessToken(h, tokenQuery, &tok); err != nil {
		return "", false
	}
	defer tok.Close()

	tu, err := tok.GetTokenUser()
	if err != nil {
		return "", false
	}
	account, domain, _, err := tu.User.Sid.LookupAccount("")
	if err != nil {
		return "", false
	}
	if domain != "" {
		return domain + "\\" + account, true
	}
	return account, true
}

// readCommandLine walks the PEB to extract RTL_USER_PROCESS_PARAMETERS.CommandLine.
// Returns ok=false if the process is WOW64, access is denied, or any read fails.
func readCommandLine(h windows.Handle) (string, bool) {
	if isWow64(h) {
		return "", false
	}
	pebAddr, ok := pebAddress(h)
	if !ok {
		return "", false
	}

	// Read ProcessParameters pointer out of the PEB.
	var ppAddr uintptr
	if !rpmUintptr(h, pebAddr+pebOffsetProcessParameters, &ppAddr) {
		return "", false
	}

	// Read the UNICODE_STRING header for CommandLine.
	var usHeader [unicodeStringSize]byte
	if !rpmBytes(h, ppAddr+rupOffsetCommandLine, usHeader[:]) {
		return "", false
	}
	length := *(*uint16)(unsafe.Pointer(&usHeader[0]))
	bufPtr := *(*uintptr)(unsafe.Pointer(&usHeader[8])) // skip Length, MaxLen, pad
	if length == 0 || bufPtr == 0 || length > 65534 {
		return "", false
	}

	// Read the string bytes.
	buf := make([]byte, length)
	if !rpmBytes(h, bufPtr, buf) {
		return "", false
	}
	u16 := bytesToUTF16(buf)
	return windows.UTF16ToString(u16), true
}

// FetchEnv extracts the env block for one process via PEB walk. Called only
// for candidates that matched the binary/cmdline heuristic.
func (l *windowsLister) FetchEnv(pid int) (map[string]string, *Warning, error) {
	h, err := windows.OpenProcess(
		windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_VM_READ,
		false, uint32(pid))
	if err != nil {
		return nil, &Warning{
			Type:    "permission_denied",
			PID:     pid,
			Message: fmt.Sprintf("OpenProcess failed for pid %d: %v (run as Administrator for full scan)", pid, err),
		}, nil
	}
	defer windows.CloseHandle(h)

	if isWow64(h) {
		return map[string]string{}, &Warning{
			Type:    "wow64_unsupported",
			PID:     pid,
			Message: fmt.Sprintf("pid %d is a 32-bit process on a 64-bit host; env read not supported in v1", pid),
		}, nil
	}

	pebAddr, ok := pebAddress(h)
	if !ok {
		return map[string]string{}, &Warning{
			Type:    "peb_unavailable",
			PID:     pid,
			Message: fmt.Sprintf("could not locate PEB for pid %d", pid),
		}, nil
	}

	// Read ProcessParameters pointer.
	var ppAddr uintptr
	if !rpmUintptr(h, pebAddr+pebOffsetProcessParameters, &ppAddr) {
		return map[string]string{}, envReadWarn(pid, "ProcessParameters"), nil
	}

	var envPtr uintptr
	if !rpmUintptr(h, ppAddr+rupOffsetEnvironment, &envPtr) {
		return map[string]string{}, envReadWarn(pid, "Environment pointer"), nil
	}
	if envPtr == 0 {
		return map[string]string{}, nil, nil
	}

	var envSize uintptr
	if !rpmUintptr(h, ppAddr+rupOffsetEnvironmentSize, &envSize) || envSize == 0 || envSize > 1<<20 {
		// Fallback: cap at 64 KB. Real env blocks are usually <32 KB.
		envSize = 64 * 1024
	}

	buf := make([]byte, envSize)
	if !rpmBytes(h, envPtr, buf) {
		return map[string]string{}, envReadWarn(pid, "environment block"), nil
	}

	return parseEnvBlock(buf), nil, nil
}

func envReadWarn(pid int, what string) *Warning {
	return &Warning{
		Type:    "peb_read_failed",
		PID:     pid,
		Message: fmt.Sprintf("ReadProcessMemory failed reading %s for pid %d", what, pid),
	}
}

// parseEnvBlock parses the Windows env block format: concatenated null-
// terminated UTF-16 KEY=VALUE strings, terminated by an extra null. Leading
// "=" entries (drive-current-directory pseudo-vars like "=C:") are skipped.
func parseEnvBlock(raw []byte) map[string]string {
	u16 := bytesToUTF16(raw)
	env := make(map[string]string)
	start := 0
	for i := 0; i < len(u16); i++ {
		if u16[i] != 0 {
			continue
		}
		if i == start {
			break // double-null: end of block
		}
		entry := windows.UTF16ToString(u16[start:i])
		start = i + 1
		if entry == "" || strings.HasPrefix(entry, "=") {
			continue
		}
		eq := strings.IndexByte(entry, '=')
		if eq <= 0 {
			continue
		}
		env[entry[:eq]] = entry[eq+1:]
	}
	return env
}

// pebAddress returns the PEB base address for a process handle.
func pebAddress(h windows.Handle) (uintptr, bool) {
	var pbi processBasicInformation
	var retLen uint32
	err := windows.NtQueryInformationProcess(
		h,
		0, // ProcessBasicInformation
		unsafe.Pointer(&pbi),
		uint32(unsafe.Sizeof(pbi)),
		&retLen,
	)
	if err != nil {
		return 0, false
	}
	if pbi.PebBaseAddress == 0 {
		return 0, false
	}
	return pbi.PebBaseAddress, true
}

func isWow64(h windows.Handle) bool {
	var b bool
	if err := windows.IsWow64Process(h, &b); err != nil {
		return false
	}
	return b
}

// rpmBytes wraps ReadProcessMemory for a byte buffer.
func rpmBytes(h windows.Handle, addr uintptr, buf []byte) bool {
	if len(buf) == 0 {
		return true
	}
	var read uintptr
	err := windows.ReadProcessMemory(h, addr, &buf[0], uintptr(len(buf)), &read)
	if err != nil {
		return false
	}
	return read > 0
}

// rpmUintptr reads a pointer-sized value from the target process.
func rpmUintptr(h windows.Handle, addr uintptr, out *uintptr) bool {
	var buf [8]byte // x64 scanner only — see package doc for WOW64 scope
	if !rpmBytes(h, addr, buf[:]) {
		return false
	}
	*out = *(*uintptr)(unsafe.Pointer(&buf[0]))
	return true
}

// bytesToUTF16 reinterprets a byte slice as a slice of uint16. Windows env
// and UNICODE_STRING buffers are UTF-16LE and always even-length on a
// little-endian host, which every supported Windows target is.
func bytesToUTF16(b []byte) []uint16 {
	n := len(b) / 2
	if n == 0 {
		return nil
	}
	return unsafe.Slice((*uint16)(unsafe.Pointer(&b[0])), n)
}

// Avoid an unused-import complaint if a future edit removes the only syscall
// reference. syscall is currently unused but kept available for WOW64 work.
var _ = syscall.Handle(0)
