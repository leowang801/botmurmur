// Package agent holds the detection heuristics and two-signal decision logic.
// All heuristic tables live here as package-level constants — no external config.
package agent

import "strings"

// CredentialSpec describes a single AI provider credential env var.
type CredentialSpec struct {
	Type     string // always "api_key" for now
	Provider string // e.g. "anthropic"
	EnvVar   string // e.g. "ANTHROPIC_API_KEY"
}

// KnownCredentials is the full table of env vars botmurmur checks for on each
// candidate process. Add new providers here AND in the detection tests.
var KnownCredentials = []CredentialSpec{
	{Type: "api_key", Provider: "anthropic", EnvVar: "ANTHROPIC_API_KEY"},
	{Type: "api_key", Provider: "openai", EnvVar: "OPENAI_API_KEY"},
	{Type: "api_key", Provider: "groq", EnvVar: "GROQ_API_KEY"},
	{Type: "api_key", Provider: "cohere", EnvVar: "COHERE_API_KEY"},
	{Type: "api_key", Provider: "huggingface", EnvVar: "HUGGINGFACE_API_TOKEN"},
	{Type: "api_key", Provider: "google", EnvVar: "GOOGLE_API_KEY"},
	// AWS Bedrock is a compound credential — we check the access key here and
	// require the secret to be present alongside it in risk flag derivation.
	{Type: "api_key", Provider: "aws", EnvVar: "AWS_ACCESS_KEY_ID"},
}

// credentialPlaceholders are lowercase tokens that indicate a developer left a
// template value in their env. We reject these as "present" to avoid false
// positives from .env.example files loaded into running processes.
var credentialPlaceholders = map[string]bool{
	"your-key-here":    true,
	"your_api_key":     true,
	"xxx":              true,
	"xxxx":             true,
	"todo":             true,
	"changeme":         true,
	"placeholder":      true,
	"<redacted>":       true,
	"<your-api-key>":   true,
	"sk-...":           true,
	"sk-xxx":           true,
}

// minCredentialLength is the floor for real API keys. Most AI provider keys
// are 32+ chars; 20 is a safe lower bound that rejects short placeholders.
const minCredentialLength = 20

// IsCredentialPresent returns true only when value passes all three checks:
//   1. length >= minCredentialLength
//   2. not in the placeholder blocklist (case-insensitive)
//   3. not composed entirely of a single repeated character
//
// This is intentionally strict. Reporting `ANTHROPIC_API_KEY=your-key-here`
// as a real credential destroys trust in the output.
func IsCredentialPresent(value string) bool {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < minCredentialLength {
		return false
	}
	if credentialPlaceholders[strings.ToLower(trimmed)] {
		return false
	}
	if isSingleCharRepeat(trimmed) {
		return false
	}
	return true
}

func isSingleCharRepeat(s string) bool {
	if len(s) == 0 {
		return false
	}
	first := s[0]
	for i := 1; i < len(s); i++ {
		if s[i] != first {
			return false
		}
	}
	return true
}

// CheckEnv returns the list of credentials with present/absent flags for the
// given env map. Nil or empty env → all absent.
func CheckEnv(env map[string]string) []CredentialResult {
	results := make([]CredentialResult, 0, len(KnownCredentials))
	for _, spec := range KnownCredentials {
		value, ok := env[spec.EnvVar]
		present := ok && IsCredentialPresent(value)
		results = append(results, CredentialResult{
			Spec:    spec,
			Present: present,
		})
	}
	return results
}

// CredentialResult is the per-spec outcome of a presence check.
type CredentialResult struct {
	Spec    CredentialSpec
	Present bool
}

// AnyPresent returns true if at least one credential was present — this is
// the AI credential signal for the two-signal detection threshold.
func AnyPresent(results []CredentialResult) bool {
	for _, r := range results {
		if r.Present {
			return true
		}
	}
	return false
}
