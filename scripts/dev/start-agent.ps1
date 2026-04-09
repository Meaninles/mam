. (Join-Path $PSScriptRoot "bootstrap-utf8.ps1")

$ErrorActionPreference = "Stop"

if (-not $env:CENTER_BASE_URL) {
  $env:CENTER_BASE_URL = "http://127.0.0.1:8080"
}

if (-not $env:AGENT_MODE) {
  $env:AGENT_MODE = "attached"
}

if (-not $env:AGENT_VERSION) {
  $env:AGENT_VERSION = "dev"
}

if (-not $env:HEARTBEAT_INTERVAL) {
  $env:HEARTBEAT_INTERVAL = "15s"
}

if (-not $env:LOG_LEVEL) {
  $env:LOG_LEVEL = "info"
}

$workspace = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$configPath = Join-Path $workspace ".air.agent.toml"

& (Join-Path $PSScriptRoot "start-go-hot.ps1") -ServiceName "agent" -ConfigPath $configPath
