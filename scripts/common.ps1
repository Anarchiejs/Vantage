# Shared helpers for Observatoire scripts (PowerShell 5.1 compatible).
# Loaded by ". (Join-Path $PSScriptRoot 'common.ps1')": it moves to the repository root.

$ErrorActionPreference = 'Stop'
# Programs (go, npm, observatoire.exe) write UTF-8. Without this, PowerShell 5.1 mangles accents.
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch {}
$OutputEncoding = [System.Text.Encoding]::UTF8

$Root = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $Root

function Step([string]$Text) { Write-Host "==> $Text" -ForegroundColor Cyan }
function Info([string]$Text) { Write-Host "    $Text" }
function Warn([string]$Text) { Write-Host "    Warning: $Text" -ForegroundColor Yellow }
function Fail([string]$Text) {
    Write-Host ''
    Write-Host "ERROR: $Text" -ForegroundColor Red
    exit 1
}

# Reload the machine and user PATH (useful right after a winget install).
function Update-Path {
    if ($env:OS -ne 'Windows_NT') { return }
    $parts = @(
        [Environment]::GetEnvironmentVariable('Path', 'Machine'),
        [Environment]::GetEnvironmentVariable('Path', 'User'),
        $env:Path
    ) | Where-Object { $_ }
    $env:Path = $parts -join ';'
}

# Run an external command and stop if it fails (non-zero exit code).
# Its error stream is shown as normal text: in PowerShell 5.1, a warning
# written to stderr (for example Vite's chunk-size warning) would otherwise become an error.
# Printed lines stay in $global:ObsOutput.
function Run([string]$Description, [scriptblock]$Command) {
    $previous = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    $global:LASTEXITCODE = -1
    $global:ObsOutput = @()
    try {
        & $Command 2>&1 | ForEach-Object {
            $line = "$_"
            $global:ObsOutput += $line
            Write-Host "    $line"
        }
        $code = $global:LASTEXITCODE
    } finally {
        $ErrorActionPreference = $previous
    }
    if ($code -ne 0) { Fail "$Description failed (code $code). Read the messages above." }
}

function Require([string]$Tool) {
    if (-not (Get-Command $Tool -ErrorAction SilentlyContinue)) {
        Fail "$Tool was not found. Run .\install.cmd first (then, if needed, close and reopen PowerShell)."
    }
}
