// Package proc defines the platform-agnostic process enumeration interface.
// Implementations live in proc_linux.go, proc_darwin.go, proc_windows.go
// behind build tags.
package proc

import "time"

// Process is the raw data returned by a platform Lister. Two-phase scans first
// enumerate processes cheaply (Env is nil), then fetch Env only for candidates
// that matched the binary/cmdline heuristic.
type Process struct {
	PID       int
	Name      string            // binary basename
	Cmd       string            // full command line
	User      string            // owning user
	StartTime time.Time         // immutable for the life of the process
	Env       map[string]string // nil until phase 2; empty map means "fetched, none"
}

// Warning is emitted for non-fatal per-PID errors (permission denied,
// truncation, antivirus blocks). The scanner bubbles these up into the
// top-level JSON warnings field.
type Warning struct {
	Type    string
	PID     int
	Message string
}

// Lister enumerates processes and fetches env vars on demand.
//
// List returns a cheap enumeration: PID, name, cmdline, user, start_time.
// Env is not populated. This is phase 1 of the two-phase scan.
//
// FetchEnv populates Env for a single process. This is phase 2 — call it
// only for candidate processes matching the binary/cmdline heuristic, to
// avoid the per-process syscall cost (especially on Windows PEB reads).
type Lister interface {
	List() ([]Process, []Warning, error)
	FetchEnv(pid int) (map[string]string, *Warning, error)
}
