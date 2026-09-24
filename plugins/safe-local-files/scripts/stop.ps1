$ErrorActionPreference = "Stop"
$pluginRoot = Split-Path -Parent $PSScriptRoot
$binaryPath = [System.IO.Path]::GetFullPath((Join-Path $pluginRoot "bin\safe-local-files.exe"))
$dataDir = Join-Path $env:LOCALAPPDATA "SafeLocalFiles"
$pidPath = Join-Path $dataDir "server.pid"

if (-not (Test-Path -LiteralPath $pidPath)) {
    Write-Output "Safe Local Files is not running."
    exit 0
}

$serverPid = [int](Get-Content -LiteralPath $pidPath -Raw)
$process = Get-Process -Id $serverPid -ErrorAction SilentlyContinue
if (-not $process) {
    Remove-Item -LiteralPath $pidPath
    Write-Output "Removed a stale PID file."
    exit 0
}

if ([System.IO.Path]::GetFullPath($process.Path) -ne $binaryPath) {
    throw "PID $serverPid does not belong to Safe Local Files; refusing to stop it."
}
Stop-Process -Id $serverPid
$process.WaitForExit(10000)
Remove-Item -LiteralPath $pidPath -ErrorAction SilentlyContinue
Write-Output "Safe Local Files stopped."
