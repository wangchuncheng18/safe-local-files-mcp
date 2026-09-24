param(
    [string]$ConfigPath = "",
    [string]$BinaryPath = ""
)

$ErrorActionPreference = "Stop"
$pluginRoot = Split-Path -Parent $PSScriptRoot
if (-not $BinaryPath) { $BinaryPath = Join-Path $pluginRoot "bin\safe-local-files.exe" }
$dataDir = Join-Path $env:LOCALAPPDATA "SafeLocalFiles"
if (-not $ConfigPath) { $ConfigPath = Join-Path $dataDir "config.json" }
$tokenPath = Join-Path $dataDir "token"
$pidPath = Join-Path $dataDir "server.pid"
$stdoutPath = Join-Path $dataDir "server.stdout.log"
$stderrPath = Join-Path $dataDir "server.stderr.log"

New-Item -ItemType Directory -Path $dataDir -Force | Out-Null
if (-not (Test-Path -LiteralPath $BinaryPath)) {
    throw "Binary not found: $BinaryPath. Run scripts\build.ps1 or install a release archive."
}

if (-not (Test-Path -LiteralPath $ConfigPath)) {
    $cfg = Get-Content -LiteralPath (Join-Path $pluginRoot "config.example.json") -Raw | ConvertFrom-Json
    $cfg.root = Join-Path $pluginRoot "example-data"
    $cfg.audit_log = Join-Path $dataDir "audit.jsonl"
    $json = $cfg | ConvertTo-Json -Depth 10
    [System.IO.File]::WriteAllText($ConfigPath, $json, (New-Object System.Text.UTF8Encoding($false)))
}

$runtimeConfig = Get-Content -LiteralPath $ConfigPath -Raw | ConvertFrom-Json
$healthHost = if ($runtimeConfig.listen -in @("0.0.0.0", "::")) { "127.0.0.1" } else { $runtimeConfig.listen }
$healthScheme = if ($runtimeConfig.tls_cert_file) { "https" } else { "http" }
$healthUri = "$healthScheme`://$healthHost`:$($runtimeConfig.port)/healthz"

$token = [Environment]::GetEnvironmentVariable("SAFE_LOCAL_FILES_TOKEN", "User")
if (-not $token -and (Test-Path -LiteralPath $tokenPath)) {
    $token = (Get-Content -LiteralPath $tokenPath -Raw).Trim()
}
if (-not $token) {
    $token = (& $BinaryPath token).Trim()
    [System.IO.File]::WriteAllText($tokenPath, $token, (New-Object System.Text.UTF8Encoding($false)))
    & icacls.exe $tokenPath /inheritance:r /grant:r "$env:USERNAME`:R" | Out-Null
}
[Environment]::SetEnvironmentVariable("SAFE_LOCAL_FILES_TOKEN", $token, "User")
$env:SAFE_LOCAL_FILES_TOKEN = $token

if (Test-Path -LiteralPath $pidPath) {
    $existingPid = [int](Get-Content -LiteralPath $pidPath -Raw)
    $existing = Get-Process -Id $existingPid -ErrorAction SilentlyContinue
    if ($existing) {
        Write-Output "Safe Local Files is already running (PID $existingPid)."
        exit 0
    }
}

& $BinaryPath validate --config $ConfigPath
$process = Start-Process -FilePath $BinaryPath -ArgumentList @("serve", "--config", "`"$ConfigPath`"") -WindowStyle Hidden -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath -PassThru
[System.IO.File]::WriteAllText($pidPath, [string]$process.Id, (New-Object System.Text.UTF8Encoding($false)))

for ($i = 0; $i -lt 20; $i++) {
    Start-Sleep -Milliseconds 250
    try {
        $health = Invoke-RestMethod -Uri $healthUri -TimeoutSec 2 -NoProxy -SkipCertificateCheck
        if ($health.status -eq "ok") {
            Write-Output "Safe Local Files started (PID $($process.Id)). Config: $ConfigPath"
            Write-Output "Restart Codex/ChatGPT once after first setup so it receives SAFE_LOCAL_FILES_TOKEN."
            exit 0
        }
    } catch {}
}
throw "Server did not become healthy. See $stderrPath"
