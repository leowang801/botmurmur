# botmurmur TODOS

Source of truth: [design doc](../../.gstack/projects/botmurmur/Leo-unknown-design-20260409-223703.md) (APPROVED, v3+)

Status legend: ⬜ not started · 🟡 in progress · ✅ done · ⛔ blocked

## Day 1 — Scaffolding (no platform-specific code yet)

- ⬜ `go mod init github.com/leo/botmurmur` (replace with actual GitHub user)
- ⬜ Create directory layout per design doc Section "Complexity check"
- ⬜ `main.go` — 30-line dispatch for `scan` and `watch` (no framework, just `os.Args`)
- ⬜ `internal/output/json.go` — full JSON schema structs with explicit `json:"..."` tags
  - `Scan`, `Agent`, `Credential`, `MCPServer`, `Warning`, `CredentialExposureSummary`
  - Use `frameworks []string` (not `framework string`)
- ⬜ `internal/agent/credentials.go` — env var → provider lookup table + presence check with placeholder blocklist + length floor
- ⬜ `internal/agent/credentials_test.go` — T5 (placeholder rejection table-driven test)
- ⬜ `internal/agent/detect.go` — heuristic tables (binary fingerprints) + two-signal decision
- ⬜ `internal/agent/detect_test.go` — T1 (two-signal truth table) + T2 (false positive suppression)
- ⬜ `internal/agent/risk.go` — 7 risk flags from the enum table
- ⬜ `internal/proc/proc.go` — shared types (`Process`, `Warning`) + `Lister` interface
- ⬜ `.gitignore` — standard Go + binary name
- ⬜ `git init`, first commit

## Day 2 — macOS lister (user's actual hardware) ✅

- ✅ Add `golang.org/x/sys` dependency
- ✅ `cmd/scan/scan.go` — platform-agnostic two-phase scan pipeline
- ✅ `internal/proc/fake.go` — FakeLister for pipeline unit tests
- ✅ `internal/proc/proc_darwin.go` — `ps -axww` enumeration + `ps eww` env fetch
- ✅ `internal/proc/parse_ps.go` — platform-neutral ps eww line parser
- ✅ `internal/proc/parse_ps_test.go` — parser tests run on every platform
- ✅ `internal/proc/proc_other.go` — loud unsupported stub for non-darwin
- ✅ `cmd/scan/scan_test.go` — 3 end-to-end pipeline tests via FakeLister
- ✅ `main.go` — wire runScan + `encoding/json` indented stdout
- ✅ Cross-compile darwin/arm64 and darwin/amd64 from Windows host
- ⬜ End-to-end `botmurmur scan` run on user's Mac hardware (user will test later)

## Day 3 — Watch + cross-platform CI ✅

- ✅ `internal/output/json.go` — add `StartTime` to `Agent` (RFC3339, watch diff key)
- ✅ `cmd/scan/scan.go` — wire `StartTime` through pipeline
- ✅ `cmd/watch/diff.go` — `(PID, start_time)` snapshot diff with stable event order
- ✅ `cmd/watch/diff_test.go` — empty / all-added / all-stopped / no-change / **PID reuse** / mixed churn
- ✅ `cmd/watch/watch.go` — `RunContext` ticker loop + SIGINT/SIGTERM via `signal.NotifyContext`
- ✅ `cmd/watch/watch_test.go` — initial-scan-emits-ADDED via FakeLister + writeEvent format lock
- ✅ `main.go` — wire `runWatch`
- ✅ `.github/workflows/ci.yml` — ubuntu/macos/windows test matrix + cross-compile job
- ⬜ End-to-end watch run on real Mac hardware (user will test later)

## Day 4 — Linux lister

- ⬜ `internal/proc/proc_linux.go` — `/proc` enumeration, cmdline read, start_time from stat field 22
- ⬜ Integration test on Linux runner via existing CI matrix

## Day 4 — MCP parsing

- ⬜ `internal/mcp/paths.go` — platform-aware path expansion
- ⬜ `internal/mcp/parse.go` — parse `mcpServers` field from JSON
- ⬜ `internal/mcp/parse_test.go` — golden files from real Cursor and Claude Desktop configs (redacted)
- ⬜ T6 (partial failure handling test with mock lister)

## Day 4 — macOS lister

- ⬜ `internal/proc/proc_darwin.go` — `ps eww` invocation, env parsing, truncation detection
- ⬜ Integration test on real macOS runner (GitHub Actions)
- ⬜ `env_truncated` warning emission test

## Day 5 — Windows lister (hardest, do last)

- ⬜ `internal/proc/proc_windows.go` — Toolhelp32 snapshot + PEB read via `ReadProcessMemory`
- ⬜ `IsWow64Process` branching for 32/64-bit bitness
- ⬜ Integration test on windows-latest runner

## Distribution

- ⬜ `.github/workflows/release.yml` — matrix build for 6 targets
- ⬜ Apple Developer account signup ($99/year) — critical path, 2-3 day buffer for notarization issues
- ⬜ Commercial code signing cert for Windows (DigiCert or Sectigo)
- ⬜ First tagged release, upload 6 binaries
- ⬜ README with install one-liners

## Gating — NOT CODE

- ⛔ **3 interviews** — blocks everything beyond Day 1 scaffolding per the design doc's gating rule. Interview kit is at `~/.gstack/projects/botmurmur/interviews/`. Currently deferred by user decision.

## Open decisions still on the table

- [ ] GitHub username / repo path for `go mod init`
- [ ] License (MIT recommended for adoption)
- [ ] Whether to ship the `single-phase` comparison mode in v1 or gate it behind a build tag
