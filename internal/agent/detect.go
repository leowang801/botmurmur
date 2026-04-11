package agent

import "strings"

// FrameworkSignature matches a process against a known agent runtime.
// A process matches when its binary basename is in BinaryNames AND (if
// CmdlineContains is non-empty) its command line contains one of the
// listed substrings. CmdlineContains empty means "binary match alone is
// enough" (used for first-party CLIs like `claude`, `cursor`).
type FrameworkSignature struct {
	Framework       string   // canonical framework name, e.g. "langchain"
	BinaryNames     []string // basenames that qualify, e.g. ["python3", "python", "python.exe"]
	CmdlineContains []string // substrings in cmdline (lowercased); empty = any cmdline
}

// KnownFrameworks is the full table of agent runtime fingerprints.
// Adding a framework here is a one-commit change that must also add a test
// case in detect_test.go covering both a positive and a negative match.
var KnownFrameworks = []FrameworkSignature{
	{
		Framework:       "langchain",
		BinaryNames:     []string{"python", "python3", "python.exe", "python3.exe"},
		CmdlineContains: []string{"langchain"},
	},
	{
		Framework:       "crewai",
		BinaryNames:     []string{"python", "python3", "python.exe", "python3.exe"},
		CmdlineContains: []string{"crewai"},
	},
	{
		Framework:       "autogen",
		BinaryNames:     []string{"python", "python3", "python.exe", "python3.exe"},
		CmdlineContains: []string{"autogen"},
	},
	{
		Framework:   "claude-code",
		BinaryNames: []string{"claude", "claude.exe"},
		// binary match alone
	},
	{
		Framework:   "cursor",
		BinaryNames: []string{"cursor", "cursor.exe"},
	},
	{
		Framework:       "js-agent",
		BinaryNames:     []string{"node", "node.exe", "bun", "bun.exe"},
		CmdlineContains: []string{"langchain", "autogen", "crewai", "ai-sdk"},
	},
}

// MatchFrameworks returns every framework that matches the given binary name
// and command line. Returns an empty slice if none match. A process can match
// multiple frameworks simultaneously (by design).
func MatchFrameworks(binaryName, cmdline string) []string {
	bn := strings.ToLower(binaryName)
	cl := strings.ToLower(cmdline)
	matched := make([]string, 0, 2)
	for _, sig := range KnownFrameworks {
		if !containsAny(bn, sig.BinaryNames) {
			continue
		}
		if len(sig.CmdlineContains) == 0 {
			matched = append(matched, sig.Framework)
			continue
		}
		for _, needle := range sig.CmdlineContains {
			if strings.Contains(cl, needle) {
				matched = append(matched, sig.Framework)
				break
			}
		}
	}
	return matched
}

func containsAny(s string, choices []string) bool {
	for _, c := range choices {
		if s == c {
			return true
		}
	}
	return false
}

// IsAgent is the two-signal detection threshold. A process is flagged as an
// agent only when BOTH conditions hold:
//   1. Its binary/cmdline matches at least one known framework signature.
//   2. At least one AI provider API key env var is present in its environment.
//
// This is the core "agent-semantic" decision. A python3 running a Flask app
// with no AI credentials is not reported. A python3 running LangChain with
// ANTHROPIC_API_KEY present is reported. Both conditions are required.
func IsAgent(frameworks []string, creds []CredentialResult) bool {
	if len(frameworks) == 0 {
		return false
	}
	return AnyPresent(creds)
}
