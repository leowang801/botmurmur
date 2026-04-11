// Package scan runs the two-phase scan pipeline over a Lister and builds a
// Scan struct. It is platform-agnostic — every platform-specific detail lives
// inside the Lister implementation.
package scan

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/leowang801/botmurmur/internal/agent"
	"github.com/leowang801/botmurmur/internal/output"
	"github.com/leowang801/botmurmur/internal/proc"
)

// Run executes a single scan: enumerate processes, match frameworks, fetch env
// for candidates, check credentials, derive risk flags, and return a populated
// Scan ready to JSON-encode.
func Run(lister proc.Lister) (output.Scan, error) {
	hostname, _ := os.Hostname()
	result := output.Scan{
		ScanTime: time.Now().UTC().Format(time.RFC3339),
		Hostname: hostname,
		Agents:   []output.Agent{},
	}

	// Phase 1: cheap enumeration
	processes, warnings, err := lister.List()
	if err != nil {
		return result, fmt.Errorf("process enumeration failed: %w", err)
	}
	for _, w := range warnings {
		result.Warnings = append(result.Warnings, output.Warning{
			Type:    w.Type,
			PID:     w.PID,
			Message: w.Message,
		})
	}

	// Phase 2: for each candidate (binary/cmdline match), fetch env and test.
	// Non-candidates never trigger an env read — this is the two-phase scan.
	providersSeen := make(map[string]bool)
	exposedCount := 0

	for _, p := range processes {
		frameworks := agent.MatchFrameworks(p.Name, p.Cmd)
		if len(frameworks) == 0 {
			continue
		}

		env, warn, err := lister.FetchEnv(p.PID)
		if warn != nil {
			result.Warnings = append(result.Warnings, output.Warning{
				Type:    warn.Type,
				PID:     warn.PID,
				Message: warn.Message,
			})
		}
		if err != nil {
			// Non-fatal: record warning and continue
			result.Warnings = append(result.Warnings, output.Warning{
				Type:    "env_fetch_failed",
				PID:     p.PID,
				Message: err.Error(),
			})
			continue
		}

		creds := agent.CheckEnv(env)
		if !agent.IsAgent(frameworks, creds) {
			continue // binary matched but no creds — not flagged
		}

		// This process is an agent. Build the record.
		credList := make([]output.Credential, 0, len(creds))
		for _, c := range creds {
			credList = append(credList, output.Credential{
				Type:     c.Spec.Type,
				Provider: c.Spec.Provider,
				EnvVar:   c.Spec.EnvVar,
				Present:  c.Present,
			})
			if c.Present {
				exposedCount++
				providersSeen[c.Spec.Provider] = true
			}
		}

		// AWS secret detection for the compound credential risk flag
		hasAWSSecret := false
		if _, ok := env["AWS_SECRET_ACCESS_KEY"]; ok {
			if agent.IsCredentialPresent(env["AWS_SECRET_ACCESS_KEY"]) {
				hasAWSSecret = true
			}
		}

		riskFlags := agent.DeriveRiskFlags(agent.RiskInput{
			Credentials:  creds,
			User:         p.User,
			HasAWSSecret: hasAWSSecret,
			// MCPServers populated in a later pass — empty for now
		})

		result.Agents = append(result.Agents, output.Agent{
			PID:         p.PID,
			Name:        p.Name,
			Frameworks:  frameworks,
			Cmd:         p.Cmd,
			User:        p.User,
			Credentials: credList,
			MCPServers:  []output.MCPServer{},
			ToolAccess:  []string{},
			RiskFlags:   riskFlags,
		})
	}

	// Summary
	providers := make([]string, 0, len(providersSeen))
	for p := range providersSeen {
		providers = append(providers, p)
	}
	sort.Strings(providers)
	result.CredentialExposureSummary = output.CredentialExposureSummary{
		ExposedKeyCount: exposedCount,
		Providers:       providers,
	}

	return result, nil
}
