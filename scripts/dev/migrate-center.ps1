. (Join-Path $PSScriptRoot "bootstrap-utf8.ps1")

$ErrorActionPreference = "Stop"

if (-not $env:DATABASE_URL) {
  $env:DATABASE_URL = "postgres://mare:mare@localhost:5432/mare_dev?sslmode=disable"
}

if (-not $env:APP_ENV) {
  $env:APP_ENV = "development"
}

if (-not $env:LOG_LEVEL) {
  $env:LOG_LEVEL = "info"
}

go run ./services/center/cmd/mare-center-migrate
