package agent

import (
	"sort"
	"testing"
)

// T1 — Two-signal detection truth table.
// This is THE test. If it breaks, the product is broken.
// A process is an agent only when binary_match AND cred_present.
func TestIsAgentTwoSignalTruthTable(t *testing.T) {
	cases := []struct {
		name       string
		binary     string
		cmdline    string
		envKey     string
		envValue   string
		wantAgent  bool
	}{
		{
			name:      "no match, no creds",
			binary:    "bash",
			cmdline:   "bash",
			wantAgent: false,
		},
		{
			name:      "binary match, no creds",
			binary:    "python3",
			cmdline:   "python3 /home/user/agent.py langchain",
			wantAgent: false, // cred signal missing
		},
		{
			name:      "no match, creds present",
			binary:    "curl",
			cmdline:   "curl https://example.com",
			envKey:    "ANTHROPIC_API_KEY",
			envValue:  "sk-ant-api03-abcdefghijklmnopqrstuvwxyz",
			wantAgent: false, // binary signal missing
		},
		{
			name:      "both signals — langchain + anthropic",
			binary:    "python3",
			cmdline:   "python3 /home/user/langchain_agent.py",
			envKey:    "ANTHROPIC_API_KEY",
			envValue:  "sk-ant-api03-abcdefghijklmnopqrstuvwxyz",
			wantAgent: true,
		},
		{
			name:      "both signals — claude code + openai",
			binary:    "claude",
			cmdline:   "claude --print hello",
			envKey:    "OPENAI_API_KEY",
			envValue:  "sk-abcdefghijklmnopqrstuvwxyz12345",
			wantAgent: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frameworks := MatchFrameworks(tc.binary, tc.cmdline)
			env := map[string]string{}
			if tc.envKey != "" {
				env[tc.envKey] = tc.envValue
			}
			creds := CheckEnv(env)
			got := IsAgent(frameworks, creds)
			if got != tc.wantAgent {
				t.Errorf("IsAgent(binary=%q, cmd=%q, env[%s]=%q) = %v, want %v",
					tc.binary, tc.cmdline, tc.envKey, tc.envValue, got, tc.wantAgent)
			}
		})
	}
}

// T2 — False positive suppression.
// Specific negative cases that must never be reported.
func TestFalsePositiveSuppression(t *testing.T) {
	cases := []struct {
		name    string
		binary  string
		cmdline string
		env     map[string]string
	}{
		{
			name:    "flask app, no AI creds",
			binary:  "python3",
			cmdline: "python3 /home/user/flask_app.py",
		},
		{
			name:    "langchain unit test, no AI creds",
			binary:  "python3",
			cmdline: "python3 -m pytest tests/test_langchain_mock.py",
			// Tricky: the cmdline contains 'langchain' so framework matches,
			// but no creds in env means NOT an agent.
		},
		{
			name:    "flask app with stray placeholder env",
			binary:  "python3",
			cmdline: "python3 /home/user/flask_app.py",
			env: map[string]string{
				"ANTHROPIC_API_KEY": "your-key-here",
			},
			// Placeholder rejected by credential presence check.
		},
		{
			name:    "bash with real-looking creds but no framework",
			binary:  "bash",
			cmdline: "bash -c 'env | grep ANTHROPIC'",
			env: map[string]string{
				"ANTHROPIC_API_KEY": "sk-ant-api03-abcdefghijklmnopqrstuvwxyz",
			},
			// Binary is not in any known framework table.
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frameworks := MatchFrameworks(tc.binary, tc.cmdline)
			creds := CheckEnv(tc.env)
			if IsAgent(frameworks, creds) {
				t.Errorf("expected NOT an agent: binary=%q cmd=%q env=%v",
					tc.binary, tc.cmdline, tc.env)
			}
		})
	}
}

func TestMatchFrameworksMultiMatch(t *testing.T) {
	// A single process can match multiple frameworks simultaneously.
	// The frameworks field is an array precisely for this case.
	frameworks := MatchFrameworks("python3", "python3 -c 'import langchain, autogen'")
	sort.Strings(frameworks)
	want := []string{"autogen", "langchain"}
	if len(frameworks) != 2 || frameworks[0] != want[0] || frameworks[1] != want[1] {
		t.Errorf("multi-match got %v, want %v", frameworks, want)
	}
}

func TestMatchFrameworksBinaryOnly(t *testing.T) {
	// Claude Code and Cursor match by binary name alone — no cmdline substring needed.
	cases := []struct {
		binary string
		want   string
	}{
		{"claude", "claude-code"},
		{"claude.exe", "claude-code"},
		{"cursor", "cursor"},
		{"cursor.exe", "cursor"},
	}
	for _, tc := range cases {
		t.Run(tc.binary, func(t *testing.T) {
			got := MatchFrameworks(tc.binary, tc.binary+" --some-flag")
			if len(got) != 1 || got[0] != tc.want {
				t.Errorf("MatchFrameworks(%q) = %v, want [%s]", tc.binary, got, tc.want)
			}
		})
	}
}
