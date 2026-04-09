param(
  [Parameter(Mandatory = $true, Position = 0)]
  [string]$ScriptName,

  [Parameter(ValueFromRemainingArguments = $true)]
  [string[]]$ScriptArgs
)

. (Join-Path $PSScriptRoot "bootstrap-utf8.ps1")

$ErrorActionPreference = "Stop"

$clientDir = Resolve-Path (Join-Path $PSScriptRoot "..\..\client")

Push-Location $clientDir

try {
  & cnpm run $ScriptName -- @ScriptArgs
  if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
  }
} finally {
  Pop-Location
}
