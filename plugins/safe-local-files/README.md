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

The first start uses `example-data`. Edit the private runtime config created by the script and set `root` to the narrowest directory that the model may read. The MCP URL is `http://127.0.0.1:8765/mcp`, authenticated with the `SAFE_LOCAL_FILES_TOKEN` environment variable.

See the repository root README, security policy, and threat model for complete installation and operational guidance.
