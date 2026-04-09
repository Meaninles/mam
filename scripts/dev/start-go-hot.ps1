param(
  [Parameter(Mandatory = $true)]
  [string]$ServiceName,

  [Parameter(Mandatory = $true)]
  [string]$ConfigPath,

  [string]$AirVersion = "v1.63.0"
)

. (Join-Path $PSScriptRoot "bootstrap-utf8.ps1")

$ErrorActionPreference = "Stop"

$workspace = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$resolvedConfigPath = (Resolve-Path $ConfigPath).Path
$toolsDir = Join-Path $workspace ".tmp\tools"
$airExe = Join-Path $toolsDir "air.exe"
$airVersionFile = Join-Path $toolsDir "air.version"

function Install-Air {
  param(
    [string]$Version
  )

  Write-Host ("Installing Air {0} for {1} hot reload..." -f $Version, $ServiceName)

  New-Item -ItemType Directory -Force -Path $toolsDir | Out-Null

  $previousGobin = $env:GOBIN

  try {
    $env:GOBIN = $toolsDir
    & go install "github.com/air-verse/air@$Version"
    if ($LASTEXITCODE -ne 0) {
      throw "go install failed with exit code $LASTEXITCODE"
    }

    Set-Content -Path $airVersionFile -Value $Version -Encoding ascii
  } finally {
    if ($null -eq $previousGobin) {
      Remove-Item Env:GOBIN -ErrorAction SilentlyContinue
    } else {
      $env:GOBIN = $previousGobin
    }
  }
}

$installedVersion = $null

if (Test-Path $airVersionFile) {
  $installedVersion = (Get-Content $airVersionFile -ErrorAction SilentlyContinue | Select-Object -First 1).Trim()
}

if (-not (Test-Path $airExe) -or $installedVersion -ne $AirVersion) {
  Install-Air -Version $AirVersion
}

Push-Location $workspace

try {
  Write-Host ("Starting {0} with hot reload..." -f $ServiceName)
  & $airExe -c $resolvedConfigPath
  if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
  }
} finally {
  Pop-Location
}
