# Install what Vantage needs to build: git, Go, Node (via winget if missing),
# then the Go and npm dependencies.
# Usage: .\install.cmd

. (Join-Path $PSScriptRoot 'common.ps1')

$tools = @(
    @{ Name = 'git';  Id = 'Git.Git' },
    @{ Name = 'go';   Id = 'GoLang.Go' },
    @{ Name = 'node'; Id = 'OpenJS.NodeJS.LTS' }
)

Step 'Checking git, Go, and Node'
foreach ($tool in $tools) {
    if (Get-Command $tool.Name -ErrorAction SilentlyContinue) {
        Info "$($tool.Name): already installed"
        continue
    }
    if (-not (Get-Command winget -ErrorAction SilentlyContinue)) {
        Fail "$($tool.Name) is missing and winget was not found. Install ""App Installer"" from the Microsoft Store (or $($tool.Name) yourself), then run this script again."
    }
    Step "Installing $($tool.Name) with winget ($($tool.Id))"
    Run "Installing $($tool.Name)" { winget install --id $tool.Id -e --silent --accept-package-agreements --accept-source-agreements }
    Update-Path
    if (-not (Get-Command $tool.Name -ErrorAction SilentlyContinue)) {
        Fail "$($tool.Name) was just installed but is still not found: close and reopen PowerShell, then run this script again."
    }
    Info "$($tool.Name): installed"
}
Require 'npm'

$nodeVersion = ("$(& node --version)").Trim().TrimStart('v')
$major = 0
[void][int]::TryParse($nodeVersion.Split('.')[0], [ref]$major)
if ($major -lt 22) {
    Warn "Node $nodeVersion detected; the UI tests need Node 22.12 or newer (winget upgrade OpenJS.NodeJS.LTS)."
}

Step 'Go dependencies (go mod download)'
Run 'go mod download' { go mod download }

if (Test-Path -LiteralPath 'web\package-lock.json') {
    Step 'UI dependencies (npm ci)'
    Run 'npm ci' { npm --prefix web ci }
} else {
    Step 'UI dependencies (npm install)'
    Run 'npm install' { npm --prefix web install }
}

Write-Host ''
Write-Host 'Installation complete.' -ForegroundColor Green
Write-Host 'Next step: .\build.cmd'
