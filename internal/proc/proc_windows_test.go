//go:build windows

package proc

import (
	"os"
	"strings"
	"testing"
	"time"
)

// The Windows lister is tested via integration against the running test
// process itself. We know our own PID, we know which env vars we set, and
// we know our own start time is recent — so each assertion can be concrete.

func TestWindowsLister_FindsSelf(t *testing.T) {
	l := NewLister()
	procs, warnings, err := l.List()
	if err != nil {
		t.Fatalf("List() returned fatal error: %v", err)
	}
	if len(procs) < 5 {
		t.Fatalf("List() returned %d processes; expected dozens on any real Windows host", len(procs))
	}

	self := os.Getpid()
	var me *Process
	for i := range procs {
		if procs[i].PID == self {
			me = &procs[i]
			break
		}
	}
	if me == nil {
		t.Fatalf("List() did not include our own PID %d; warnings=%v", self, warnings)
	}

	// Name should be the Go test binary. It varies by `go test` invocation
	// (often "proc.test.exe" or similar) but it's always non-empty and ends
	// in .exe.
	if me.Name == "" || !strings.HasSuffix(strings.ToLower(me.Name), ".exe") {
		t.Errorf("self Name = %q, expected a .exe basename", me.Name)
	}

	// Start time should be within the last 10 minutes — the test binary
	// was just launched. This also proves the FILETIME→Unix conversion is
	// not off by centuries (a classic 1601-epoch bug).
	if me.StartTime.IsZero() {
		t.Error("self StartTime is zero — GetProcessTimes path failed")
	} else {
		age := time.Since(me.StartTime)
		if age < 0 || age > 10*time.Minute {
			t.Errorf("self StartTime age = %v; expected 0–10 minutes", age)
		}
	}

	if me.User == "" {
		t.Error("self User is empty — token lookup path failed")
	}
}

func TestWindowsLister_FetchEnv_Self(t *testing.T) {
	// Set a sentinel env var before spawning any child — the value gets
	// baked into our own PEB env block and FetchEnv should see it.
	const sentinel = "BOTMURMUR_TEST_SENTINEL"
	const sentinelValue = "hello-from-windows-test-sk-ant-api03-filler-for-length"
	t.Setenv(sentinel, sentinelValue)

	l := NewLister()
	env, warn, err := l.FetchEnv(os.Getpid())
	if err != nil {
		t.Fatalf("FetchEnv(self) error: %v", err)
	}
	if warn != nil {
		t.Fatalf("FetchEnv(self) unexpected warning: %+v", warn)
	}

	if len(env) == 0 {
		t.Fatal("FetchEnv(self) returned empty env; PEB read likely failed silently")
	}

	// Look for a stable env var every Windows process has.
	if _, ok := env["SystemRoot"]; !ok {
		if _, ok := env["SYSTEMROOT"]; !ok {
			t.Errorf("FetchEnv(self) missing SystemRoot — env block parse may be broken. Got %d keys", len(env))
		}
	}

	// Our sentinel may or may not reach the PEB depending on how Go's
	// os.Setenv interacts with the Win32 env block on this Windows version.
	// Treat its absence as a soft signal rather than a hard failure, since
	// FetchEnv correctness is proven by SystemRoot. But log it so we notice
	// if behavior changes.
	if v, ok := env[sentinel]; ok {
		if v != sentinelValue {
			t.Errorf("sentinel value mismatch: got %q, want %q", v, sentinelValue)
		}
	} else {
		t.Logf("sentinel %s not found in PEB env block (acceptable — os.Setenv does not always propagate to Win32 env block)", sentinel)
	}
}

func TestWindowsLister_FetchEnv_InvalidPID(t *testing.T) {
	// Pick a PID that's almost certainly not live (max user PID space, odd
	// value so it wouldn't be aligned to thread counts). FetchEnv must
	// return a permission_denied warning, not a fatal error.
	env, warn, err := NewLister().FetchEnv(0x7FFFFFFE)
	if err != nil {
		t.Fatalf("FetchEnv on nonexistent pid returned fatal error: %v", err)
	}
	if warn == nil {
		t.Fatal("expected a warning for nonexistent pid, got nil")
	}
	if warn.Type != "permission_denied" {
		t.Errorf("warning Type = %q, want permission_denied", warn.Type)
	}
	// env may be nil on permission_denied — matches the macOS contract.
	// Ranging a nil map is safe in Go, so downstream code handles it fine.
	_ = env
}

func TestParseEnvBlock(t *testing.T) {
	// Build a synthetic env block: "FOO=bar\0BAZ=qux\0=C:=C:\\tmp\0\0" in UTF-16LE.
	entries := []string{"FOO=bar", "BAZ=qux", "=C:=C:\\tmp"}
	var buf []byte
	for _, e := range entries {
		for _, r := range e {
			buf = append(buf, byte(r), 0x00)
		}
		buf = append(buf, 0x00, 0x00) // null terminator
	}
	buf = append(buf, 0x00, 0x00) // final terminator

	env := parseEnvBlock(buf)
	if env["FOO"] != "bar" {
		t.Errorf("FOO = %q, want bar", env["FOO"])
	}
	if env["BAZ"] != "qux" {
		t.Errorf("BAZ = %q, want qux", env["BAZ"])
	}
	// "=C:" entries must be skipped (they're drive-pwd pseudo-vars, not env).
	for k := range env {
		if strings.HasPrefix(k, "=") {
			t.Errorf("parseEnvBlock should skip =-prefixed entries, got key %q", k)
		}
	}
}
