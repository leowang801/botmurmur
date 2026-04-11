package agent

import "testing"

// T5 — Credential placeholder rejection.
// A stray ANTHROPIC_API_KEY=your-key-here in a process env must NOT count
// as "present". Every row here is a regression against the false-positive
// cases that destroy trust in the output.
func TestIsCredentialPresent(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  bool
	}{
		// Real-looking keys
		{"anthropic-like", "sk-ant-api03-abcdefghijklmnopqrstuvwxyz1234567890", true},
		{"openai-like", "sk-abcdefghijklmnopqrstuvwxyz1234567890", true},
		{"aws-access-key-like", "AKIAIOSFODNN7EXAMPLE1234", true},

		// Placeholders — must be rejected
		{"template-english", "your-key-here", false},
		{"template-snake", "your_api_key", false},
		{"xxx-short", "xxx", false},
		{"xxxx-short", "xxxx", false},
		{"todo", "todo", false},
		{"changeme", "changeme", false},
		{"placeholder", "placeholder", false},
		{"redacted-tag", "<redacted>", false},
		{"angle-template", "<your-api-key>", false},
		{"sk-ellipsis", "sk-...", false},
		{"sk-xxx", "sk-xxx", false},
		{"template-uppercase", "YOUR-KEY-HERE", false},

		// Length floor
		{"empty", "", false},
		{"single-char", "a", false},
		{"19-chars", "abcdefghijklmnopqrs", false},
		{"20-chars", "abcdefghijklmnopqrst", true},

		// Single-char repeat (zero-entropy string)
		{"all-a", "aaaaaaaaaaaaaaaaaaaaaaaaaaaa", false},
		{"all-x", "xxxxxxxxxxxxxxxxxxxxxxxxxxxx", false},

		// Whitespace handling — trimmed
		{"padded-real", "  sk-ant-api03-abcdefghijklmnopqrstuvwxyz  ", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsCredentialPresent(tc.value)
			if got != tc.want {
				t.Errorf("IsCredentialPresent(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestCheckEnvAnyPresent(t *testing.T) {
	t.Run("nil env", func(t *testing.T) {
		results := CheckEnv(nil)
		if AnyPresent(results) {
			t.Error("nil env should not have any credentials present")
		}
	})

	t.Run("empty env", func(t *testing.T) {
		results := CheckEnv(map[string]string{})
		if AnyPresent(results) {
			t.Error("empty env should not have any credentials present")
		}
	})

	t.Run("placeholder only", func(t *testing.T) {
		results := CheckEnv(map[string]string{
			"ANTHROPIC_API_KEY": "your-key-here",
		})
		if AnyPresent(results) {
			t.Error("placeholder env should not count as present")
		}
	})

	t.Run("real anthropic key", func(t *testing.T) {
		results := CheckEnv(map[string]string{
			"ANTHROPIC_API_KEY": "sk-ant-api03-abcdefghijklmnopqrstuvwxyz",
		})
		if !AnyPresent(results) {
			t.Error("real anthropic key should count as present")
		}
	})

	t.Run("unrelated env vars", func(t *testing.T) {
		results := CheckEnv(map[string]string{
			"PATH":   "/usr/bin:/bin",
			"HOME":   "/home/leo",
			"EDITOR": "vim",
		})
		if AnyPresent(results) {
			t.Error("unrelated env should not have any credentials present")
		}
	})
}
