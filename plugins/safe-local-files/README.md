# Safe Local Files plugin

This directory contains the portable plugin, Codex compatibility manifest, Go MCP server, configuration example, and start/stop scripts.

Build and start on Windows:

```powershell
.\scripts\build.ps1
.\scripts\start.ps1
```

Build and start on macOS/Linux:

```sh
chmod +x scripts/*.sh
./scripts/build.sh
./scripts/start.sh
```

The first start uses `example-data`. Edit the private runtime config created by the script and set `root` to the narrowest directory that the model may access. The optional MCP HTTP URL is `http://127.0.0.1:47381/mcp`, authenticated with the `SAFE_LOCAL_FILES_TOKEN` environment variable. The local Codex installer registers `safe-local-files stdio --config <path>` so the client starts and stops the server with the conversation and no HTTP listener is required. Writes are disabled unless separately enabled in the private configuration.

See the repository root README, security policy, and threat model for complete installation and operational guidance.

For a stable ChatGPT connection through Cloudflare Tunnel and Access Managed OAuth, see the repository-root `CLOUDFLARE.md`. Each user needs a domain in their own Cloudflare account and configures their own root directory; Quick Tunnel's random URL is only suitable for temporary tests.
