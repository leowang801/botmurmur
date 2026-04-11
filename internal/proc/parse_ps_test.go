package proc

import (
	"strings"
	"testing"
)

// These tests verify the `ps eww` line parser. They build on real-looking
// examples captured from macOS, with AI credentials padded to pass the
// presence check in agent.IsCredentialPresent.
//
// No build tag: the parser lives in parse_ps.go and is pure string
// manipulation, so the tests run on every platform.

func TestParsePsEwwLine(t *testing.T) {
	realAnthropic := "sk-ant-api03-abcdefghijklmnopqrstuvwxyz1234567890"

	cases := []struct {
		name    string
		line    string
		want    map[string]string
		wantKey string // a key that must be in the parsed env
		trunc   bool
	}{
		{
			name: "simple python agent with one AI key",
			line: "python3 /Users/leo/agent.py PATH=/usr/bin:/bin HOME=/Users/leo ANTHROPIC_API_KEY=" + realAnthropic,
			want: map[string]string{
				"PATH":              "/usr/bin:/bin",
				"HOME":              "/Users/leo",
				"ANTHROPIC_API_KEY": realAnthropic,
			},
			wantKey: "ANTHROPIC_API_KEY",
		},
		{
			name: "argv with equals-containing flag is not confused for env",
			line: "node --inspect=9229 /Users/leo/server.js NODE_ENV=production OPENAI_API_KEY=" + realAnthropic,
			want: map[string]string{
				"NODE_ENV":       "production",
				"OPENAI_API_KEY": realAnthropic,
			},
			// The --inspect=9229 token has a leading `--` so envVarKey won't
			// match — the parser stops scanning env at that point.
			wantKey: "OPENAI_API_KEY",
		},
		{
			name:    "no env vars at all",
			line:    "python3 -c print('hello')",
			want:    map[string]string{},
			wantKey: "",
		},
		{
			name:    "pure env after bare binary",
			line:    "claude FOO=bar BAZ=qux",
			want:    map[string]string{"FOO": "bar", "BAZ": "qux"},
			wantKey: "FOO",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env, truncated := parsePsEwwLine(tc.line)
			if truncated != tc.trunc {
				t.Errorf("truncated = %v, want %v", truncated, tc.trunc)
			}
			if tc.wantKey != "" {
				if _, ok := env[tc.wantKey]; !ok {
					t.Errorf("expected key %q in parsed env, got keys: %v",
						tc.wantKey, keysOf(env))
				}
			}
			for k, v := range tc.want {
				if got := env[k]; got != v {
					t.Errorf("env[%q] = %q, want %q", k, got, v)
				}
			}
		})
	}
}

func TestParsePsEwwLineTruncationHeuristic(t *testing.T) {
	// Build a line >=4000 chars where the final env token has a short value,
	// matching the pattern ps eww produces when the kernel clamps the env
	// block mid-value. The heuristic should fire.
	padding := strings.Repeat("a", 4000)
	line := "python3 " + padding + " SOMEVAR=x"
	if len(line) < 4000 {
		t.Fatalf("test setup bug: line length %d is below threshold 4000", len(line))
	}
	_, trunc := parsePsEwwLine(line)
	if !trunc {
		t.Error("expected truncation warning for short trailing env value at >=4000 chars")
	}

	// Negative case: same length, but the final env value is long enough to
	// look like a real complete token. Should NOT be flagged as truncated.
	realValue := strings.Repeat("k", 40)
	okLine := "python3 " + strings.Repeat("a", 4000) + " SOMEVAR=" + realValue
	_, trunc2 := parsePsEwwLine(okLine)
	if trunc2 {
		t.Error("did not expect truncation warning for complete final env value")
	}
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
