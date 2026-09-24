$ErrorActionPreference = "Stop"
$dataDir = Join-Path $env:LOCALAPPDATA "SafeLocalFiles"
$pidPath = Join-Path $dataDir "cloudflared.pid"
if (-not (Test-Path -LiteralPath $pidPath)) {
    Write-Output "No managed cloudflared process recorded."
    exit 0
}
$tunnelPid = [int](Get-Content -LiteralPath $pidPath -Raw)
$process = Get-Process -Id $tunnelPid -ErrorAction SilentlyContinue
if ($process -and $process.ProcessName -eq "cloudflared") {
    Stop-Process -Id $tunnelPid
}
Remove-Item -LiteralPath $pidPath
Write-Output "Managed cloudflared process stopped."
