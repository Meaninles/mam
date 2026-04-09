. (Join-Path $PSScriptRoot "bootstrap-utf8.ps1")

$ErrorActionPreference = "Stop"

$composeFile = Join-Path $PSScriptRoot "..\..\infra\dev\docker-compose.yml"
docker compose -f $composeFile down
