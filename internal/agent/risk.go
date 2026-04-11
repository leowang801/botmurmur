package agent

// Risk flag string constants. These are the only valid values emitted in the
// JSON `risk_flags` field. New flags must be added here, in DeriveRiskFlags,
// and in risk_test.go in the same commit.
const (
	FlagCredentialInEnv      = "credential_in_env"
	FlagMultipleProviders    = "multiple_providers"
	FlagFilesystemMCPServer  = "filesystem_mcp_server"
	FlagShellMCPServer       = "shell_mcp_server"
	FlagNetworkMCPServer     = "network_mcp_server"
	FlagAWSCredentialsPresent = "aws_credentials_present"
	FlagRootUser             = "root_user"
)

// RiskInput bundles the post-detection data that risk flag derivation needs.
type RiskInput struct {
	Credentials []CredentialResult
	MCPServers  []string // server type names: "filesystem", "shell", "fetch", etc.
	User        string
	HasAWSSecret bool // AWS_SECRET_ACCESS_KEY was also present
}

// DeriveRiskFlags returns the complete set of risk flags for a detected agent.
// Order is deterministic (matches the const declaration order above) to keep
// JSON output stable and diff-friendly.
func DeriveRiskFlags(in RiskInput) []string {
	flags := make([]string, 0, 4)

	if AnyPresent(in.Credentials) {
		flags = append(flags, FlagCredentialInEnv)
	}
	if countDistinctProviders(in.Credentials) > 1 {
		flags = append(flags, FlagMultipleProviders)
	}
	for _, server := range in.MCPServers {
		switch server {
		case "filesystem":
			flags = append(flags, FlagFilesystemMCPServer)
		case "shell", "exec", "bash":
			flags = append(flags, FlagShellMCPServer)
		case "fetch", "http", "web":
			flags = append(flags, FlagNetworkMCPServer)
		}
	}
	if in.HasAWSSecret && credProviderPresent(in.Credentials, "aws") {
		flags = append(flags, FlagAWSCredentialsPresent)
	}
	if in.User == "root" || in.User == "Administrator" || in.User == "SYSTEM" {
		flags = append(flags, FlagRootUser)
	}
	return flags
}

func countDistinctProviders(creds []CredentialResult) int {
	seen := make(map[string]bool)
	for _, c := range creds {
		if c.Present {
			seen[c.Spec.Provider] = true
		}
	}
	return len(seen)
}

func credProviderPresent(creds []CredentialResult, provider string) bool {
	for _, c := range creds {
		if c.Present && c.Spec.Provider == provider {
			return true
		}
	}
	return false
}
