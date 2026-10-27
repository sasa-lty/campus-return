param([string]$Distribution = 'Ubuntu-D', [string]$FabricSamplesPath = '')
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
if (-not (Test-Path -LiteralPath (Join-Path $projectRoot 'frontend/dist/index.html'))) {
  Push-Location (Join-Path $projectRoot 'frontend')
  try { npm ci --no-audit --no-fund; if ($LASTEXITCODE -ne 0) { throw 'npm ci failed' }; npm run build; if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed' } } finally { Pop-Location }
}
$linuxRoot = (& wsl.exe -d $Distribution -- wslpath -u $projectRoot).Trim()
if ($FabricSamplesPath) {
  & wsl.exe -d $Distribution -- env "FABRIC_SAMPLES_PATH=$FabricSamplesPath" bash "$linuxRoot/scripts/start.sh"
} else {
  & wsl.exe -d $Distribution -- bash "$linuxRoot/scripts/start.sh"
}
if ($LASTEXITCODE -ne 0) { throw "WSL startup failed: $LASTEXITCODE" }
