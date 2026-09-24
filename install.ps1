$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$pluginRoot = Join-Path $repoRoot "plugins\safe-local-files"

& (Join-Path $pluginRoot "scripts\build.ps1")
codex plugin marketplace add $repoRoot
codex plugin add safe-local-files@personal
& (Join-Path $pluginRoot "scripts\start.ps1")

Write-Output "Installed Safe Local Files. Restart the ChatGPT desktop app and use a new task."
