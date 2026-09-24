param(
    [string]$TokenFile = "",
    [string]$ConfigPath = ""
)

$ErrorActionPreference = "Stop"
$dataDir = Join-Path $env:LOCALAPPDATA "SafeLocalFiles"
if (-not $TokenFile) { $TokenFile = Join-Path $dataDir "cloudflare-tunnel-token" }
if (-not $ConfigPath) { $ConfigPath = Join-Path $dataDir "config.json" }
$pidPath = Join-Path $dataDir "cloudflared.pid"
$logPath = Join-Path $dataDir "cloudflared.stderr.log"
$stdoutPath = Join-Path $dataDir "cloudflared.stdout.log"
$binary = (Get-Command cloudflared.exe -ErrorAction SilentlyContinue).Source
if (-not $binary) { $binary = "C:\Program Files (x86)\cloudflared\cloudflared.exe" }
if (-not (Test-Path -LiteralPath $binary)) { throw "cloudflared.exe is not installed" }
if (-not (Test-Path -LiteralPath $TokenFile)) { throw "Tunnel token file is missing: $TokenFile" }
if (-not (Test-Path -LiteralPath $ConfigPath)) { throw "MCP config is missing: $ConfigPath" }
$cfg = Get-Content -LiteralPath $ConfigPath -Raw | ConvertFrom-Json
if ($cfg.auth_mode -ne "cloudflare_access" -or -not $cfg.behind_proxy -or $cfg.listen -notin @("127.0.0.1", "::1")) {
    throw "Cloudflare mode requires auth_mode=cloudflare_access, behind_proxy=true and loopback listen"
}
if (Test-Path -LiteralPath $pidPath) {
    $oldPid = [int](Get-Content -LiteralPath $pidPath -Raw)
    $old = Get-Process -Id $oldPid -ErrorAction SilentlyContinue
    if ($old -and $old.Path -eq $binary) {
        Write-Output "cloudflared is already running (PID $oldPid)"
        exit 0
    }
}
& (Join-Path $PSScriptRoot "start.ps1") -ConfigPath $ConfigPath
$process = Start-Process -FilePath $binary -ArgumentList @("tunnel", "run", "--token-file", "`"$TokenFile`"") -WindowStyle Hidden -RedirectStandardOutput $stdoutPath -RedirectStandardError $logPath -PassThru
[System.IO.File]::WriteAllText($pidPath, [string]$process.Id, (New-Object System.Text.UTF8Encoding($false)))
Write-Output "cloudflared started (PID $($process.Id)); check Cloudflare dashboard for tunnel health."
