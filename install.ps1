$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$pluginRoot = Join-Path $repoRoot "plugins\safe-local-files"

& (Join-Path $pluginRoot "scripts\build.ps1")
codex plugin marketplace add $repoRoot
codex plugin add safe-local-files@personal
& (Join-Path $pluginRoot "scripts\start.ps1")
& (Join-Path $pluginRoot "scripts\stop.ps1")
$binaryPath = Join-Path $pluginRoot "bin\safe-local-files.exe"
$configPath = Join-Path $env:LOCALAPPDATA "SafeLocalFiles\config.json"
codex mcp remove safe_local_files 2>$null | Out-Null
codex mcp add safe_local_files -- $binaryPath stdio --config $configPath

Write-Output "Installed Safe Local Files with on-demand stdio MCP. Start a new task."
