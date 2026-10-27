$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$toolDirectory = Join-Path $projectRoot '.runtime'
$toolPath = Join-Path $toolDirectory 'cloudflared.exe'
New-Item -ItemType Directory -Path $toolDirectory -Force | Out-Null
if (-not (Test-Path -LiteralPath $toolPath)) {
  Invoke-WebRequest -Uri 'https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-windows-amd64.exe' -OutFile $toolPath -TimeoutSec 120
}
Invoke-WebRequest -Uri 'http://127.0.0.1:8088/api/v1/health' -UseBasicParsing | Out-Null
& $toolPath tunnel --url http://127.0.0.1:8088 --protocol http2 --edge-ip-version 4
