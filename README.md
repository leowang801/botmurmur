# botmurmur

Agent-semantic process scanner. Given a running process, `botmurmur` answers:

- Is this an AI agent? Which framework?
- What provider credentials does it hold in env vars (presence only, never values)?
- What MCP servers and tools does it have access to?
- What risk flags apply (root-owned, multi-provider creds, cloud creds in same env)?

Output is stable JSON, designed for security teams who want to inventory AI activity across a fleet.

Status: **pre-alpha.** macOS and Windows listers work. Linux lister lands in a follow-up commit.

## Commands

```
botmurmur scan     # one-shot JSON inventory to stdout
botmurmur watch    # poll every 30s, print one line per added/stopped agent
botmurmur version  # print version
botmurmur help     # usage
```

## Install

### From source (macOS, Linux, Windows)

Requires Go **1.26.2** or newer.

```bash
git clone https://github.com/leowang801/botmurmur.git
cd botmurmur
go build -o botmurmur .
```

Put the resulting binary anywhere on your `PATH`.

### Cross-compile from any host

`botmurmur` is pure Go with no CGO, so you can build a macOS binary from a Windows host or vice versa:

```bash
GOOS=darwin  GOARCH=arm64 go build -o botmurmur-darwin-arm64 .
GOOS=darwin  GOARCH=amd64 go build -o botmurmur-darwin-amd64 .
GOOS=linux   GOARCH=amd64 go build -o botmurmur-linux-amd64 .
GOOS=windows GOARCH=amd64 go build -o botmurmur-windows-amd64.exe .
```

## Running on macOS

The macOS lister shells out to `/bin/ps` — no private syscalls, no kext, no CGO. Run as your normal user to see processes you own:

```bash
./botmurmur scan
./botmurmur watch
```

To see processes owned by other users (the whole machine), run with `sudo`:

```bash
sudo ./botmurmur scan
```

Known limits on macOS:

- `ps eww` truncates the env block at ~4096 chars on many macOS versions. When this happens, the JSON output includes an `env_truncated` warning for the affected PID and detection still runs on whatever was captured.
- Without `sudo`, you only see processes owned by the current user.

Watching in a terminal:

```bash
./botmurmur watch
# 2026-04-11T10:23:45Z added pid=42831 frameworks=[claude-code] credentials=[ANTHROPIC_API_KEY] user=leo
# 2026-04-11T10:24:12Z stopped pid=42831 frameworks=[claude-code] credentials=[] user=leo
# ^C to exit
```

## Running on Windows

The Windows lister uses `CreateToolhelp32Snapshot` for enumeration and PEB reads via `ReadProcessMemory` for command lines and environment variables. Pure Go, no CGO, no external tools, statically linked.

```bash
go build -o botmurmur.exe .
./botmurmur.exe scan
./botmurmur.exe watch
```

Run as a normal user to see processes you own. To inspect processes owned by other users (SYSTEM services, other logons), launch an **elevated** terminal (Run as Administrator) and run from there.

Known limits on Windows v1:

- **32-bit target processes on a 64-bit host (WOW64):** env read is skipped and a `wow64_unsupported` warning is emitted. The 32-bit PEB lives at a different virtual address with different struct offsets. Full WOW64 support lands in a follow-up commit.
- **Protected processes** (antivirus, DRM, lsass, csrss): `OpenProcess` is refused by the kernel regardless of elevation. `permission_denied` warning is emitted and the scan continues. This is working as intended — a userspace tool cannot read the env of a PP/PPL process.
- **Command-line reading** requires `PROCESS_VM_READ`. Without it, the `cmd` field falls back to the executable basename, which means script-language agents (Python + LangChain, Node + ai-sdk) won't match their framework unless the scanner is elevated.

PowerShell equivalents for env vars in cross-compile commands:

```powershell
$env:GOOS="darwin"; $env:GOARCH="arm64"; go build -o botmurmur-darwin-arm64.exe .
```

## JSON output schema

`botmurmur scan` emits:

```json
{
  "scan_time": "2026-04-11T10:23:45Z",
  "hostname": "leo-mbp",
  "agents": [
    {
      "pid": 42831,
      "name": "claude",
      "frameworks": ["claude-code"],
      "cmd": "claude --interactive",
      "user": "leo",
      "start_time": "2026-04-11T10:00:00Z",
      "credentials": [
        {"type": "api_key", "provider": "anthropic", "env_var": "ANTHROPIC_API_KEY", "present": true}
      ],
      "mcp_servers": [],
      "tool_access": [],
      "risk_flags": []
    }
  ],
  "credential_exposure_summary": {"exposed_key_count": 1, "providers": ["anthropic"]},
  "warnings": []
}
```

The schema is stable — field names are load-bearing for downstream consumers. Breaking changes bump the major version.

## Privacy

`botmurmur` **never reads or logs credential values.** For every known provider env var, it records only `present: true` or `present: false`. A presence check requires the value to pass a length floor and a placeholder blocklist — so empty strings, `"changeme"`, or `"xxxxx"` don't count as exposed credentials.

## Development

```bash
go test ./... -count=1   # run full test suite
go vet ./...             # static checks
```

CI runs the matrix (`ubuntu-latest`, `macos-latest`, `windows-latest`) plus a cross-compile sanity job on every push and pull request. See [.github/workflows/ci.yml](.github/workflows/ci.yml).

## License

TBD — pending license decision. Treat as all-rights-reserved until a LICENSE file lands.
