# PowerShell 5.1 defaults to legacy Windows code pages. Force UTF-8 for the
# current process so Chinese text remains stable across scripts and child tools.
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)

[Console]::InputEncoding = $utf8NoBom
[Console]::OutputEncoding = $utf8NoBom
$global:OutputEncoding = $utf8NoBom

$PSDefaultParameterValues['Out-File:Encoding'] = 'utf8'
$PSDefaultParameterValues['Set-Content:Encoding'] = 'utf8'
$PSDefaultParameterValues['Add-Content:Encoding'] = 'utf8'
$PSDefaultParameterValues['Export-Csv:Encoding'] = 'utf8'

$env:PYTHONUTF8 = "1"
$env:LANG = "C.UTF-8"
$env:LC_ALL = "C.UTF-8"

try {
  & "$env:SystemRoot\System32\chcp.com" 65001 > $null
} catch {
}
