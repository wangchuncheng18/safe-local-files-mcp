param([switch]$RestartHttp)

$ErrorActionPreference = "Stop"
$pluginRoot = Split-Path -Parent $PSScriptRoot
$binary = Join-Path $pluginRoot "bin\safe-local-files.exe"
$dataDir = Join-Path $env:LOCALAPPDATA "SafeLocalFiles"
$tokenPath = Join-Path $dataDir "token"
if (-not (Test-Path -LiteralPath $binary)) { throw "Build the server before rotating the token." }
New-Item -ItemType Directory -Path $dataDir -Force | Out-Null
if ($RestartHttp) { & (Join-Path $PSScriptRoot "stop.ps1") }
$newToken = (& $binary token).Trim()
if ($LASTEXITCODE -ne 0 -or $newToken.Length -lt 32) { throw "Token generation failed." }
$tempPath = Join-Path $dataDir "token.new"
[IO.File]::WriteAllText($tempPath, $newToken, (New-Object Text.UTF8Encoding($false)))
& icacls.exe $tempPath /inheritance:r /grant:r "$env:USERNAME`:R" | Out-Null
Move-Item -LiteralPath $tempPath -Destination $tokenPath -Force
[Environment]::SetEnvironmentVariable("SAFE_LOCAL_FILES_TOKEN", $newToken, "User")
$env:SAFE_LOCAL_FILES_TOKEN = $newToken
if ($RestartHttp) { & (Join-Path $PSScriptRoot "start.ps1") }
Write-Output "HTTP token rotated. Restart HTTP clients to pick up the new value. The token was not printed."
