package proc

import (
	"regexp"
	"strings"
)

// envVarKey matches a valid environment variable key: starts with a letter or
// underscore, followed by letters, digits, or underscores. AI credential env
// vars are conventionally uppercase but we accept both to be safe.
var envVarKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// parsePsEwwLine extracts environment variables from a `ps eww -o command=`
// line. Returns the env map and a boolean indicating whether the output
// appears truncated (macOS ps clamps env output at ~4096 chars).
//
// Parsing strategy: `ps eww` emits `argv0 argv1 ... envk=v envk2=v2 ...` on a
// single line. We cannot unambiguously split argv from env in general, so we
// take each trailing space-separated token that looks like KEY=VALUE with a
// valid env-var KEY and stop at the first token that doesn't. This is the
// standard heuristic used by gopsutil and other cross-platform tools; it
// handles every real AI credential env var we care about.
//
// Lives in a build-tag-free file so the parser can be unit-tested on every
// platform. proc_darwin.go is the only caller.
func parsePsEwwLine(line string) (map[string]string, bool) {
	env := make(map[string]string)
	tokens := strings.Fields(line)

	// Walk from the end, collecting trailing KEY=VALUE tokens. Stop at the
	// first token that doesn't look like an env var — everything before that
	// is argv.
	firstEnvIdx := len(tokens)
	for i := len(tokens) - 1; i >= 0; i-- {
		eq := strings.Index(tokens[i], "=")
		if eq <= 0 {
			break
		}
		key := tokens[i][:eq]
		if !envVarKey.MatchString(key) {
			break
		}
		firstEnvIdx = i
	}

	for _, tok := range tokens[firstEnvIdx:] {
		eq := strings.Index(tok, "=")
		env[tok[:eq]] = tok[eq+1:]
	}

	// Truncation heuristic: the kernel exec block is typically 4096 bytes. If
	// the raw line is close to that limit and the final env token has no
	// value or an unusually short one, consider the output truncated.
	truncated := false
	if len(line) >= 4000 {
		if firstEnvIdx < len(tokens) {
			last := tokens[len(tokens)-1]
			if eq := strings.Index(last, "="); eq < 0 || len(last)-eq-1 < 5 {
				truncated = true
			}
		}
	}
	return env, truncated
}
