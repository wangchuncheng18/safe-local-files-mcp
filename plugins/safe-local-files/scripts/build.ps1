param([string]$OutputDirectory = "")

$ErrorActionPreference = "Stop"
$pluginRoot = Split-Path -Parent $PSScriptRoot
if (-not $OutputDirectory) { $OutputDirectory = Join-Path $pluginRoot "bin" }
New-Item -ItemType Directory -Path $OutputDirectory -Force | Out-Null
$env:CGO_ENABLED = "0"
Push-Location $pluginRoot
try {
    go build -trimpath -ldflags "-s -w" -o (Join-Path $OutputDirectory "safe-local-files.exe") ./cmd/safe-local-files
} finally {
    Pop-Location
}
Write-Output "Built $(Join-Path $OutputDirectory 'safe-local-files.exe')"
