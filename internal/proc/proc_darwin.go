//go:build darwin

package proc

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The ps eww line parser (parsePsEwwLine) lives in parse_ps.go so it can be
// unit-tested on every platform — the parser is pure string manipulation.

// NewLister returns the macOS process lister. It shells out to /bin/ps — no
// CGO, no private syscalls. This keeps the binary statically linked and the
// code obvious to audit.
//
// Known limitations (documented in the design doc):
//   - `ps eww` truncates env output at ~4096 chars on many macOS versions.
//     Truncated results emit an `env_truncated` warning; detection still runs
//     on whatever was captured.
//   - Cross-user inspection requires elevated privileges. Without sudo, the
//     scanner only sees processes owned by the current user (enforced by ps).
func NewLister() Lister {
	return &darwinLister{}
}

type darwinLister struct{}

// lstartLayout matches the output of `ps -o lstart` on macOS. Note the
// space-padded day: "Thu Apr 10 09:05:33 2026" (single-digit days use a
// leading space, not a leading zero).
const lstartLayout = "Mon Jan _2 15:04:05 2006"

// psRowRegexp extracts a single process row from `ps -axww` output. We ask
// ps for a deterministic column order with trailing `=` headers suppressed:
//
//	ps -axww -o pid=,user=,lstart=,comm=,command=
//
// lstart is always 24 chars (e.g. "Thu Apr 10 09:05:33 2026"). comm is the
// basename of the executable, command is the full argv. Parsing is
// whitespace-delimited up to comm, then the rest of the line is command.
var psRowRegexp = regexp.MustCompile(`^\s*(\d+)\s+(\S+)\s+(\S+ \S+ [\s\d]\d \d{2}:\d{2}:\d{2} \d{4})\s+(\S+)\s+(.*)$`)

// List enumerates processes using a single `ps` call. This is phase 1 of the
// two-phase scan: no env vars are fetched here.
func (l *darwinLister) List() ([]Process, []Warning, error) {
	cmd := exec.Command("ps", "-axww", "-o", "pid=,user=,lstart=,comm=,command=")
	out, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("ps enumeration failed: %w", err)
	}

	var procs []Process
	var warnings []Warning
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := psRowRegexp.FindStringSubmatch(line)
		if m == nil {
			// Silently skip — some ps lines on macOS have odd formatting for
			// kernel threads and aren't worth a warning each.
			continue
		}
		pid, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		user := m[2]
		startTime, err := time.ParseInLocation(lstartLayout, m[3], time.Local)
		if err != nil {
			warnings = append(warnings, Warning{
				Type:    "lstart_parse_failed",
				PID:     pid,
				Message: fmt.Sprintf("could not parse lstart %q: %v", m[3], err),
			})
			continue
		}
		comm := filepath.Base(m[4])
		command := m[5]

		procs = append(procs, Process{
			PID:       pid,
			Name:      comm,
			Cmd:       command,
			User:      user,
			StartTime: startTime,
		})
	}

	return procs, warnings, nil
}

// FetchEnv reads the environment for one process via `ps eww`. This is phase
// 2 — called only for processes that matched the binary/cmdline heuristic, so
// the per-process subprocess cost is paid on a small number of candidates.
//
// Parsing strategy: `ps eww -p PID -o command=` emits `argv0 argv1 ... envk=v
// envk2=v2 ...` on a single line. We cannot unambiguously split argv from env
// in general, so we take the pragmatic approach used by gopsutil and others:
// take each trailing space-separated token that looks like KEY=VALUE with a
// valid env-var KEY (uppercase letters, digits, underscores, starting with a
// letter or underscore). This is the standard heuristic and it handles every
// real AI credential env var we care about.
//
// Truncation detection: macOS ps clamps the exec arg+env block at ~4096 chars
// for many processes. We emit an `env_truncated` warning when the raw output
// is suspiciously close to that limit AND the final token doesn't look like
// a complete KEY=VALUE pair.
func (l *darwinLister) FetchEnv(pid int) (map[string]string, *Warning, error) {
	cmd := exec.Command("ps", "eww", "-p", strconv.Itoa(pid), "-o", "command=")
	out, err := cmd.Output()
	if err != nil {
		return nil, &Warning{
			Type:    "permission_denied",
			PID:     pid,
			Message: fmt.Sprintf("cannot read env for pid %d: %v (run with sudo for full scan)", pid, err),
		}, nil
	}

	line := strings.TrimRight(string(out), "\n")
	if line == "" {
		return map[string]string{}, nil, nil
	}

	env, truncated := parsePsEwwLine(line)

	if truncated {
		return env, &Warning{
			Type:    "env_truncated",
			PID:     pid,
			Message: fmt.Sprintf("ps eww output for pid %d appears truncated at ~4096 chars; some credentials may not be detected", pid),
		}, nil
	}
	return env, nil, nil
}

