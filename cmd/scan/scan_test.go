package scan

import (
	"errors"
	"testing"
	"time"

	"github.com/leowang801/botmurmur/internal/proc"
)

const realAnthropicKey = "sk-ant-api03-abcdefghijklmnopqrstuvwxyz1234567890"
const realOpenAIKey = "sk-openaiabcdefghijklmnopqrstuvwxyz1234567890"

func TestScanPipelineEndToEnd(t *testing.T) {
	now := time.Now()
	lister := &proc.FakeLister{
		Processes: []proc.Process{
			{PID: 100, Name: "bash", Cmd: "bash", User: "leo", StartTime: now},
			{PID: 200, Name: "python3", Cmd: "python3 /home/leo/flask_app.py", User: "leo", StartTime: now},
			{PID: 300, Name: "python3", Cmd: "python3 /home/leo/langchain_agent.py", User: "leo", StartTime: now},
			{PID: 400, Name: "claude", Cmd: "claude --print hello", User: "leo", StartTime: now},
		},
		Envs: map[int]map[string]string{
			// bash has a real key but no framework match — must NOT be flagged
			100: {"ANTHROPIC_API_KEY": realAnthropicKey},
			// flask has framework match on python3 but no creds — must NOT be flagged
			200: {"FLASK_ENV": "production"},
			// langchain process with a real anthropic key — MUST be flagged
			300: {"ANTHROPIC_API_KEY": realAnthropicKey},
			// claude with an openai key — MUST be flagged
			400: {"OPENAI_API_KEY": realOpenAIKey},
		},
	}

	result, err := Run(lister)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(result.Agents) != 2 {
		t.Fatalf("expected 2 agents detected, got %d: %+v", len(result.Agents), result.Agents)
	}

	// Verify PID 100 (bash) and 200 (flask) are NOT present
	for _, a := range result.Agents {
		if a.PID == 100 {
			t.Error("bash with stray anthropic key must not be flagged — no framework match")
		}
		if a.PID == 200 {
			t.Error("flask app with no creds must not be flagged — no credential signal")
		}
	}

	// Verify the credential exposure summary
	if result.CredentialExposureSummary.ExposedKeyCount != 2 {
		t.Errorf("expected 2 exposed keys, got %d", result.CredentialExposureSummary.ExposedKeyCount)
	}
	wantProviders := map[string]bool{"anthropic": true, "openai": true}
	if len(result.CredentialExposureSummary.Providers) != 2 {
		t.Errorf("expected 2 providers, got %v", result.CredentialExposureSummary.Providers)
	}
	for _, p := range result.CredentialExposureSummary.Providers {
		if !wantProviders[p] {
			t.Errorf("unexpected provider %q in summary", p)
		}
	}
}

func TestScanWithFetchEnvError(t *testing.T) {
	// When FetchEnv fails for a candidate, the scan must record a warning
	// and continue — it must not abort the whole scan.
	lister := &proc.FakeLister{
		Processes: []proc.Process{
			{PID: 100, Name: "python3", Cmd: "python3 langchain_agent.py", User: "leo"},
			{PID: 200, Name: "claude", Cmd: "claude --print hi", User: "leo"},
		},
		Envs: map[int]map[string]string{
			200: {"ANTHROPIC_API_KEY": realAnthropicKey},
		},
		FetchErrors: map[int]error{
			100: errors.New("simulated permission denied"),
		},
	}

	result, err := Run(lister)
	if err != nil {
		t.Fatalf("scan should not fail on per-PID errors: %v", err)
	}
	if len(result.Agents) != 1 || result.Agents[0].PID != 200 {
		t.Errorf("expected 1 agent (pid 200), got %+v", result.Agents)
	}
	if len(result.Warnings) == 0 {
		t.Error("expected at least one warning for the failed FetchEnv")
	}
	foundWarning := false
	for _, w := range result.Warnings {
		if w.PID == 100 && w.Type == "env_fetch_failed" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected env_fetch_failed warning for pid 100, got %+v", result.Warnings)
	}
}

func TestScanNoCandidates(t *testing.T) {
	// A machine with no AI-related processes must produce a valid, empty scan.
	lister := &proc.FakeLister{
		Processes: []proc.Process{
			{PID: 100, Name: "bash", Cmd: "bash"},
			{PID: 200, Name: "sshd", Cmd: "/usr/sbin/sshd -D"},
			{PID: 300, Name: "vim", Cmd: "vim /etc/hosts"},
		},
	}
	result, err := Run(lister)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(result.Agents))
	}
	if result.CredentialExposureSummary.ExposedKeyCount != 0 {
		t.Errorf("expected 0 exposed keys, got %d", result.CredentialExposureSummary.ExposedKeyCount)
	}
	if result.Hostname == "" {
		t.Error("hostname must be populated even with no agents")
	}
	if result.ScanTime == "" {
		t.Error("scan_time must be populated even with no agents")
	}
}
