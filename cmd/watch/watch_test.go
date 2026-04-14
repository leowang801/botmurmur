package watch

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/leowang801/botmurmur/internal/output"
	"github.com/leowang801/botmurmur/internal/proc"
)

// TestRunContext_InitialScanEmitsAdded verifies that processes already running
// at startup are reported as ADDED on the first tick. Without this, an
// operator who starts watching after agents are already up would see nothing.
func TestRunContext_InitialScanEmitsAdded(t *testing.T) {
	lister := &proc.FakeLister{
		Processes: []proc.Process{
			{
				PID:       42,
				Name:      "claude",
				Cmd:       "claude --interactive",
				User:      "leo",
				StartTime: time.Date(2026, 4, 11, 10, 0, 0, 0, time.UTC),
			},
		},
		Envs: map[int]map[string]string{
			42: {"ANTHROPIC_API_KEY": "sk-ant-api03-abcdefghijklmnopqrstuvwxyz1234567890"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so the loop runs the initial scan and exits before
	// the first ticker fire.
	cancel()

	var buf bytes.Buffer
	if err := RunContext(ctx, lister, 100*time.Millisecond, &buf); err != nil {
		t.Fatalf("RunContext returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, " added pid=42") {
		t.Errorf("expected ADDED event for pid 42, got:\n%s", out)
	}
	if !strings.Contains(out, "frameworks=[claude-code]") {
		t.Errorf("expected claude-code framework in output, got:\n%s", out)
	}
	if !strings.Contains(out, "credentials=[ANTHROPIC_API_KEY]") {
		t.Errorf("expected ANTHROPIC_API_KEY credential in output, got:\n%s", out)
	}
}

func TestWriteEventFormat(t *testing.T) {
	e := Event{
		Kind: EventAdded,
		Agent: output.Agent{
			PID:        99,
			Frameworks: []string{"claude-code", "langchain"},
			User:       "leo",
			Credentials: []output.Credential{
				{EnvVar: "ANTHROPIC_API_KEY", Present: true},
				{EnvVar: "OPENAI_API_KEY", Present: false},
			},
		},
	}
	var buf bytes.Buffer
	writeEvent(&buf, time.Date(2026, 4, 11, 10, 23, 45, 0, time.UTC), e)
	got := buf.String()
	want := "2026-04-11T10:23:45Z added pid=99 frameworks=[claude-code,langchain] credentials=[ANTHROPIC_API_KEY] user=leo\n"
	if got != want {
		t.Errorf("event line mismatch\n got: %q\nwant: %q", got, want)
	}
}
