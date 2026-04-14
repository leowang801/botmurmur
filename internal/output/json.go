// Package output defines the stable JSON schema for botmurmur scan/watch output.
// Field names here are load-bearing — downstream consumers will grep on them.
// Changing a field name is a breaking change.
package output

// Scan is the top-level object emitted by `botmurmur scan`.
type Scan struct {
	ScanTime                  string                    `json:"scan_time"`
	Hostname                  string                    `json:"hostname"`
	Agents                    []Agent                   `json:"agents"`
	CredentialExposureSummary CredentialExposureSummary `json:"credential_exposure_summary"`
	Warnings                  []Warning                 `json:"warnings,omitempty"`
}

// Agent is a single detected AI agent process.
// Frameworks is always an array, even when only one framework matched. This
// schema choice is intentional — a process can match multiple frameworks
// simultaneously (e.g. Claude Code invoking a LangChain subprocess tool).
//
// StartTime is RFC3339-encoded process start time. It is part of the process
// identity used by `botmurmur watch` to diff snapshots: (PID, StartTime) is
// stable even across PID reuse.
type Agent struct {
	PID         int          `json:"pid"`
	Name        string       `json:"name"`
	Frameworks  []string     `json:"frameworks"`
	Cmd         string       `json:"cmd"`
	User        string       `json:"user"`
	StartTime   string       `json:"start_time"`
	Credentials []Credential `json:"credentials"`
	MCPServers  []MCPServer  `json:"mcp_servers"`
	ToolAccess  []string     `json:"tool_access"`
	RiskFlags   []string     `json:"risk_flags"`
}

// Credential describes the presence or absence of an AI provider API key in a
// process's environment. Values are never read or logged — only presence.
type Credential struct {
	Type     string `json:"type"`
	Provider string `json:"provider"`
	EnvVar   string `json:"env_var"`
	Present  bool   `json:"present"`
}

// MCPServer is an entry parsed from a local MCP config file.
type MCPServer struct {
	Name       string `json:"name"`
	ConfigPath string `json:"config_path"`
}

// CredentialExposureSummary rolls up credential presence across all agents.
type CredentialExposureSummary struct {
	ExposedKeyCount int      `json:"exposed_key_count"`
	Providers       []string `json:"providers"`
}

// Warning describes a non-fatal problem encountered during a scan.
// Scans emit warnings inline and exit 0; partial results are still useful.
type Warning struct {
	Type    string `json:"type"`
	PID     int    `json:"pid,omitempty"`
	Message string `json:"message"`
}
