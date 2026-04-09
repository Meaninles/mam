param(
  [switch]$RestartDatabase
)

. (Join-Path $PSScriptRoot "scripts\dev\bootstrap-utf8.ps1")

$ErrorActionPreference = "Stop"

$workspace = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $workspace

function Stop-MareProcesses {
  Write-Host "Stopping existing center, agent, and client processes..."

  $patterns = @(
    'services/center/cmd/mare-center',
    'mare-center',
    'start-center\.ps1',
    '\.air\.center\.toml',
    'start-go-hot\.ps1.*center',
    'services/agent/cmd/mare-agent',
    'mare-agent',
    'start-agent\.ps1',
    '\.air\.agent\.toml',
    'start-go-hot\.ps1.*agent',
    'mare_client\.exe',
    'tauri:dev',
    'run-client-script\.ps1 tauri:dev',
    'client\\src-tauri\\Cargo\.toml',
    'webview-exe-name=mare_client\.exe'
  )

  $processes = Get-CimInstance Win32_Process | Where-Object {
    $commandLine = $_.CommandLine
    if (-not $commandLine) {
      return $false
    }

    foreach ($pattern in $patterns) {
      if ($commandLine -match $pattern) {
        return $true
      }
    }

    return $false
  }

  foreach ($process in $processes) {
    try {
      Stop-Process -Id $process.ProcessId -Force -ErrorAction Stop
      Write-Host ("Stopped process PID={0} Name={1}" -f $process.ProcessId, $process.Name)
    } catch {
      Write-Warning ("Failed to stop process PID={0} Name={1}: {2}" -f $process.ProcessId, $process.Name, $_.Exception.Message)
    }
  }

  Start-Sleep -Seconds 2
}

function Wait-ForCenterReady {
  param(
    [string]$BaseUrl,
    [int]$TimeoutSeconds = 90
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    try {
      $response = Invoke-RestMethod "$BaseUrl/readyz"
      if ($response.data.status -eq 'ready' -and $response.data.database.status -eq 'up' -and $response.data.migration.status -eq 'ready') {
        Write-Host "Center is ready."
        return $true
      }
    } catch {
    }

    Start-Sleep -Seconds 2
  }

  return $false
}

function Wait-ForAgentOnline {
  param(
    [string]$BaseUrl,
    [int]$TimeoutSeconds = 60
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  while ((Get-Date) -lt $deadline) {
    try {
      $response = Invoke-RestMethod "$BaseUrl/api/runtime/status"
      $agents = @($response.data.agents)
      if ($agents.Count -gt 0 -and ($agents | Where-Object { $_.status -eq 'online' })) {
        Write-Host "Agent is online."
        return $true
      }
    } catch {
    }

    Start-Sleep -Seconds 2
  }

  return $false
}

function Start-DetachedPowerShell {
  param(
    [string]$Title,
    [string]$Command
  )

  $bootstrapPath = (Join-Path $workspace "scripts\dev\bootstrap-utf8.ps1") -replace "'", "''"
  $escapedTitle = $Title -replace "'", "''"
  $escapedWorkspace = $workspace -replace "'", "''"
  $escapedCommand = $Command -replace "'", "''"

  $argumentList = @(
    '-NoLogo',
    '-NoProfile',
    '-NoExit',
    '-ExecutionPolicy', 'Bypass',
    '-Command',
    "& { . '$bootstrapPath'; `$host.UI.RawUI.WindowTitle='$escapedTitle'; Set-Location '$escapedWorkspace'; $escapedCommand }"
  )

  Start-Process -FilePath 'powershell.exe' -ArgumentList $argumentList | Out-Null
}

$centerBaseUrl = 'http://127.0.0.1:8080'

Stop-MareProcesses

if ($RestartDatabase) {
  Write-Host "Restarting development PostgreSQL..."
  powershell -NoLogo -NoProfile -ExecutionPolicy Bypass -File "$workspace\scripts\dev\down-postgres.ps1"
} else {
  Write-Host "Keeping database state and ensuring PostgreSQL is running..."
}

powershell -NoLogo -NoProfile -ExecutionPolicy Bypass -File "$workspace\scripts\dev\up-postgres.ps1"

Write-Host "Starting center..."
Start-DetachedPowerShell -Title 'MARE Center' -Command "cnpm run center:dev"

if (-not (Wait-ForCenterReady -BaseUrl $centerBaseUrl)) {
  throw "Center did not enter ready state before timeout. Check the center terminal output."
}

Write-Host "Starting agent..."
Start-DetachedPowerShell -Title 'MARE Agent' -Command "cnpm run agent:dev"

if (-not (Wait-ForAgentOnline -BaseUrl $centerBaseUrl)) {
  Write-Warning "Agent did not become online before timeout. Client will still start, but the status light may not be green."
}

Write-Host "Starting client..."
Start-DetachedPowerShell -Title 'MARE Client' -Command "cnpm run tauri:dev"

Write-Host "One link start completed."
