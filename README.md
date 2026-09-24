# Safe Local Files MCP

Safe Local Files is a cross-platform, read-only MCP server and Codex/ChatGPT plugin for querying one explicitly configured local directory. It is designed for least privilege, bounded resource use, secret avoidance, and auditability.

The repository is also a Codex marketplace. The plugin source is in [`plugins/safe-local-files`](plugins/safe-local-files).

## Security defaults

- Read-only tools only: `search`, `fetch`, `list_directory`, `read_file`, and `stat_path`.
- One canonical root directory; traversal outside it is rejected.
- Symlinks and junction-like paths are rejected by default.
- Loopback-only listener (`127.0.0.1`) unless remote binding is explicitly enabled.
- Bearer token required; the token comes from `SAFE_LOCAL_FILES_TOKEN`, never from a committed config file.
- Sensitive names such as `.env`, private keys, credentials, cloud configuration, and Git metadata are denied by default.
- Secret-like content is denied by default and redacted in search previews.
- File size, response size, search duration, scanned file count, result count, concurrency, request body, and request rate are bounded.
- JSONL audit records contain tool, relative path, outcome, byte/result counts, duration, and query hashes. Tokens and file contents are never logged.

See [SECURITY.md](SECURITY.md) and [THREAT_MODEL.md](THREAT_MODEL.md) before using the server with business data.

## Quick start

### Windows

```powershell
cd plugins\safe-local-files
.\scripts\build.ps1
.\scripts\start.ps1
```

On the first run, the script creates `%LOCALAPPDATA%\SafeLocalFiles\config.json`, generates a token without printing it, stores the token with user-only ACLs, and sets the user environment variable used by the plugin. Edit the `root` in that private config to the exact directory you want to expose, then restart the server:

```powershell
.\scripts\stop.ps1
.\scripts\start.ps1
```

### macOS and Linux

```sh
cd plugins/safe-local-files
chmod +x scripts/*.sh
./scripts/build.sh
./scripts/start.sh
```

The first run creates `~/.config/safe-local-files/config.json` and a mode-`0600` token file. Edit the `root`, then stop and start the service.

## Install the local Codex plugin

From the repository root:

```sh
codex plugin marketplace add .
codex plugin add safe-local-files@personal
```

Restart the ChatGPT desktop app and start a new task after installation. Existing tasks do not reload plugin tools. The bundled MCP connection reads its bearer token from `SAFE_LOCAL_FILES_TOKEN`.

The default MCP URL is `http://127.0.0.1:8765/mcp`. The binary also supports `stdio` so a local Codex client can launch it on demand without depending on a background port. If you change the HTTP port, update both the private server config and these plugin files before reinstalling:

- `plugins/safe-local-files/mcp.json`
- `plugins/safe-local-files/.mcp.json`

For the most reliable local Codex setup, register the binary as an on-demand stdio server after building it. Replace the paths for your platform:

```powershell
codex mcp add safe_local_files -- "E:\work\safe-local-files-mcp\plugins\safe-local-files\bin\safe-local-files.exe" stdio --config "$env:LOCALAPPDATA\SafeLocalFiles\config.json"
```

With stdio registration, Codex owns the server process and no background HTTP listener is needed. Start a new task after changing MCP registration.

Plugin-scoped tool policy can further restrict discovery:

```toml
[plugins."safe-local-files".mcp_servers.safe_local_files]
enabled = true
default_tools_approval_mode = "prompt"
enabled_tools = ["search", "fetch", "list_directory", "read_file", "stat_path"]
```

## Configuration

Private runtime configuration is JSON. Copy [`config.example.json`](plugins/safe-local-files/config.example.json) or let the start script create it.

| Field | Default | Purpose |
| --- | --- | --- |
| `root` | sample directory | Only directory visible through MCP |
| `listen` | `127.0.0.1` | Bind address |
| `port` | `8765` | HTTP port |
| `token_env` | `SAFE_LOCAL_FILES_TOKEN` | Environment variable containing the bearer token |
| `audit_log` | local data directory | Append-only JSONL audit destination |
| `allow_remote` | `false` | Required before binding to a non-loopback address |
| `follow_symlinks` | `false` | Whether resolved symlinks may be followed inside the root |
| `deny_globs` | secure defaults | Files and directories that cannot be accessed |
| `allow_extensions` | text formats | File extensions eligible for reading/searching |
| `max_file_bytes` | 1 MiB | Maximum input file size |
| `max_response_bytes` | 256 KiB | Maximum file content returned per call |
| `max_search_files` | 10,000 | Maximum files scanned per search |
| `max_search_results` | 100 | Maximum matches per search |
| `search_timeout_ms` | 2,000 | Search deadline |
| `max_concurrency` | 4 | Concurrent authenticated HTTP requests |
| `requests_per_minute` | 60 | Per-client request cap |
| `sensitive_content_action` | `deny` | `deny` or `redact` for direct reads |

Environment overrides are available for deployment: `SLFM_ROOT`, `SLFM_LISTEN`, `SLFM_PORT`, `SLFM_TOKEN_ENV`, `SLFM_AUDIT_LOG`, and `SLFM_CONFIG`.

## Performance model

The service does no background indexing or filesystem watching. It opens files only during an authenticated tool call and stops at configured time, file, byte, and result limits. This avoids steady-state CPU and I/O load on production workstations.

## Compatibility

Release builds target Windows, macOS, and Linux on AMD64 and ARM64. The server supports stdio and Streamable HTTP MCP using the official Go SDK. Stdio is preferred for local Codex use. Localhost HTTP access requires a local/self-hosted Codex or ChatGPT environment; a cloud-only browser session cannot directly reach your machine's `127.0.0.1`.

## Development

```sh
cd plugins/safe-local-files
go test ./...
go vet ./...
```

The GitHub release workflow builds stripped, CGO-free binaries and publishes SHA-256 checksums.

## License

MIT
