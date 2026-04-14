package watch

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/leowang801/botmurmur/cmd/scan"
	"github.com/leowang801/botmurmur/internal/output"
	"github.com/leowang801/botmurmur/internal/proc"
)

// DefaultInterval is the polling interval for `botmurmur watch`.
const DefaultInterval = 30 * time.Second

// Run starts the watch loop. It scans on a fixed interval, diffs each new
// snapshot against the previous one, and emits one human-readable line per
// event to out. The loop exits cleanly on SIGINT or SIGTERM.
//
// The lister is taken as a parameter so tests can swap in a FakeLister and
// drive the loop deterministically.
func Run(lister proc.Lister, interval time.Duration, out io.Writer) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return RunContext(ctx, lister, interval, out)
}

// RunContext is the testable core of Run. It exits when ctx is canceled.
func RunContext(ctx context.Context, lister proc.Lister, interval time.Duration, out io.Writer) error {
	if interval <= 0 {
		interval = DefaultInterval
	}

	// Initial scan establishes the baseline. Anything already running at
	// startup is reported as ADDED so the operator sees the full picture
	// without having to wait for churn.
	prev, err := scanOnce(lister)
	if err != nil {
		return fmt.Errorf("initial scan failed: %w", err)
	}
	for _, e := range Diff(nil, prev) {
		writeEvent(out, time.Now().UTC(), e)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			curr, err := scanOnce(lister)
			if err != nil {
				// Single scan failures are non-fatal — log and keep going.
				// A transient ps failure shouldn't kill a long-running watcher.
				fmt.Fprintf(out, "%s scan_error %v\n",
					time.Now().UTC().Format(time.RFC3339), err)
				continue
			}
			for _, e := range Diff(prev, curr) {
				writeEvent(out, time.Now().UTC(), e)
			}
			prev = curr
		}
	}
}

// scanOnce wraps scan.Run and returns just the agents slice.
func scanOnce(lister proc.Lister) ([]output.Agent, error) {
	result, err := scan.Run(lister)
	if err != nil {
		return nil, err
	}
	return result.Agents, nil
}

// writeEvent emits one line per event in a stable, grep-friendly format:
//
//	2026-04-11T10:23:45Z added pid=42 frameworks=[claude-code] credentials=[ANTHROPIC_API_KEY] user=leo
//	2026-04-11T10:24:12Z stopped pid=42 frameworks=[claude-code] user=leo
func writeEvent(out io.Writer, t time.Time, e Event) {
	creds := make([]string, 0, len(e.Agent.Credentials))
	for _, c := range e.Agent.Credentials {
		if c.Present {
			creds = append(creds, c.EnvVar)
		}
	}
	fmt.Fprintf(out, "%s %s pid=%d frameworks=[%s] credentials=[%s] user=%s\n",
		t.Format(time.RFC3339),
		string(e.Kind),
		e.Agent.PID,
		strings.Join(e.Agent.Frameworks, ","),
		strings.Join(creds, ","),
		e.Agent.User,
	)
}
