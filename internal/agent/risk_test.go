package agent

import (
	"reflect"
	"testing"
)

func TestDeriveRiskFlags(t *testing.T) {
	realAnthropic := map[string]string{
		"ANTHROPIC_API_KEY": "sk-ant-api03-abcdefghijklmnopqrstuvwxyz",
	}
	anthropicAndOpenAI := map[string]string{
		"ANTHROPIC_API_KEY": "sk-ant-api03-abcdefghijklmnopqrstuvwxyz",
		"OPENAI_API_KEY":    "sk-openaiabcdefghijklmnopqrstuvwxyz",
	}
	awsFull := map[string]string{
		"AWS_ACCESS_KEY_ID": "AKIAIOSFODNN7EXAMPLE1234",
	}

	cases := []struct {
		name      string
		env       map[string]string
		mcpTypes  []string
		user      string
		awsSecret bool
		want      []string
	}{
		{
			name: "single credential only",
			env:  realAnthropic,
			want: []string{FlagCredentialInEnv},
		},
		{
			name: "multiple providers",
			env:  anthropicAndOpenAI,
			want: []string{FlagCredentialInEnv, FlagMultipleProviders},
		},
		{
			name:     "filesystem MCP",
			env:      realAnthropic,
			mcpTypes: []string{"filesystem"},
			want:     []string{FlagCredentialInEnv, FlagFilesystemMCPServer},
		},
		{
			name:     "shell MCP",
			env:      realAnthropic,
			mcpTypes: []string{"shell"},
			want:     []string{FlagCredentialInEnv, FlagShellMCPServer},
		},
		{
			name:     "network MCP (fetch)",
			env:      realAnthropic,
			mcpTypes: []string{"fetch"},
			want:     []string{FlagCredentialInEnv, FlagNetworkMCPServer},
		},
		{
			name:      "aws compound credential",
			env:       awsFull,
			awsSecret: true,
			want:      []string{FlagCredentialInEnv, FlagAWSCredentialsPresent},
		},
		{
			name: "root user",
			env:  realAnthropic,
			user: "root",
			want: []string{FlagCredentialInEnv, FlagRootUser},
		},
		{
			name: "administrator on windows",
			env:  realAnthropic,
			user: "Administrator",
			want: []string{FlagCredentialInEnv, FlagRootUser},
		},
		{
			name: "non-root user no extra flags",
			env:  realAnthropic,
			user: "leo",
			want: []string{FlagCredentialInEnv},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := RiskInput{
				Credentials:  CheckEnv(tc.env),
				MCPServers:   tc.mcpTypes,
				User:         tc.user,
				HasAWSSecret: tc.awsSecret,
			}
			got := DeriveRiskFlags(in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("DeriveRiskFlags = %v, want %v", got, tc.want)
			}
		})
	}
}
