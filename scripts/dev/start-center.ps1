$ErrorActionPreference = "Stop"

if (-not $env:DATABASE_URL) {
  $env:DATABASE_URL = "postgres://mare:mare@localhost:5432/mare_dev?sslmode=disable"
}

if (-not $env:HTTP_ADDR) {
  $env:HTTP_ADDR = ":8080"
}

if (-not $env:APP_ENV) {
  $env:APP_ENV = "development"
}

if (-not $env:LOG_LEVEL) {
  $env:LOG_LEVEL = "info"
}

if (-not $env:SERVICE_VERSION) {
  $env:SERVICE_VERSION = "dev"
}

go run ./services/center/cmd/mare-center
