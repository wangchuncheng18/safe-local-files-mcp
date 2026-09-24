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
$mcpBinary = Join-Path (Split-Path -Parent $PSScriptRoot) "bin\safe-local-files.exe"
$binary = (Get-Command cloudflared.exe -ErrorAction SilentlyContinue).Source
if (-not $binary) { $binary = "C:\Program Files (x86)\cloudflared\cloudflared.exe" }
if (-not (Test-Path -LiteralPath $binary)) { throw "cloudflared.exe is not installed" }
if (-not (Test-Path -LiteralPath $TokenFile)) { throw "Tunnel token file is missing: $TokenFile" }
if (-not (Test-Path -LiteralPath $ConfigPath)) { throw "MCP config is missing: $ConfigPath" }
if (-not (Test-Path -LiteralPath $mcpBinary)) { throw "MCP binary is missing: $mcpBinary" }
& $mcpBinary validate --config $ConfigPath --require-cloudflare-read-only
if ($LASTEXITCODE -ne 0) { throw "Cloudflare read-only preflight failed" }
if (Test-Path -LiteralPath $pidPath) {
    $oldPid = [int](Get-Content -LiteralPath $pidPath -Raw)
    $old = Get-Process -Id $oldPid -ErrorAction SilentlyContinue
    if ($old -and $old.Path -eq $binary) {
        Write-Output "cloudflared is already running (PID $oldPid)"
        exit 0
    }
}
& (Join-Path $PSScriptRoot "start.ps1") -ConfigPath $ConfigPath
$cfg = Get-Content -LiteralPath $ConfigPath -Raw | ConvertFrom-Json
Add-Type -AssemblyName System.Net.Http
$handler = [System.Net.Http.HttpClientHandler]::new()
$handler.UseProxy = $false
$client = [System.Net.Http.HttpClient]::new($handler)
$client.Timeout = [TimeSpan]::FromSeconds(5)
try {
    $body = [System.Net.Http.StringContent]::new('{}', [System.Text.Encoding]::UTF8, 'application/json')
    try {
        $response = $client.PostAsync("http://127.0.0.1:$($cfg.port)/mcp", $body).GetAwaiter().GetResult()
        try {
            if ([int]$response.StatusCode -ne 401) { throw "Unauthenticated MCP preflight expected HTTP 401, got $([int]$response.StatusCode)" }
        } finally { $response.Dispose() }
    } finally { $body.Dispose() }
} finally { $client.Dispose(); $handler.Dispose() }
$process = Start-Process -FilePath $binary -ArgumentList @("tunnel", "run", "--token-file", "`"$TokenFile`"") -WindowStyle Hidden -RedirectStandardOutput $stdoutPath -RedirectStandardError $logPath -PassThru
[System.IO.File]::WriteAllText($pidPath, [string]$process.Id, (New-Object System.Text.UTF8Encoding($false)))
Write-Output "cloudflared started (PID $($process.Id)); check Cloudflare dashboard for tunnel health."
