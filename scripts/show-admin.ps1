param([string]$Distribution = 'Ubuntu-D')
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$linuxRoot = (& wsl.exe -d $Distribution -- wslpath -u $projectRoot).Trim()
& wsl.exe -d $Distribution -- bash "$linuxRoot/scripts/admin-info.sh"
