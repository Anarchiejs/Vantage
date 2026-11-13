# Import your Obsidian vault, then serve the UI (http://127.0.0.1:7878).
# Usage: .\start.cmd "C:\path\to\your vault"   (the first time)
#        .\start.cmd                            (later: reuse the last vault)
#        .\start.cmd -Port 8080 -Repos "Name=C:\path" "C:\path\to\your vault"
# Ctrl+C stops the server.
# -Port and -Repos match ./start.sh --port and --repos.

param(
    [Parameter(Position = 0)]
    [string]$Vault,
    [int]$Port = 7878,
    [string]$Repos = ''
)

$memo = Join-Path $env:LOCALAPPDATA 'Vantage\vault.txt'

if ($Port -lt 1 -or $Port -gt 65535) {
    Write-Host ''
    Write-Host 'ERROR: port must be a number between 1 and 65535.' -ForegroundColor Red
    exit 1
}

$legacyMemo = Join-Path $env:LOCALAPPDATA 'Observatoire\vault.txt'
if (-not (Test-Path -LiteralPath $memo) -and (Test-Path -LiteralPath $legacyMemo)) {
    Write-Host "warning: $legacyMemo is the previous remembered vault. Move that file to $memo"
}

if (-not $Vault) {
    if (Test-Path -LiteralPath $memo) {
        $Vault = (Get-Content -LiteralPath $memo -TotalCount 1).Trim()
        Write-Host "Remembered vault: $Vault"
    } elseif (Test-Path -LiteralPath $legacyMemo) {
        $Vault = (Get-Content -LiteralPath $legacyMemo -TotalCount 1).Trim()
        Write-Host "Remembered vault: $Vault"
    } else {
        Write-Host ''
        Write-Host 'ERROR: give your Obsidian vault folder the first time, for example:' -ForegroundColor Red
        Write-Host '  .\start.cmd "C:\Users\you\Documents\My Vault"'
        exit 1
    }
}

# Resolve the path before leaving the current directory: a relative path stays valid.
if (-not (Test-Path -LiteralPath $Vault -PathType Container)) {
    Write-Host ''
    Write-Host "ERROR: folder not found: $Vault (give your Obsidian vault folder, in quotes)." -ForegroundColor Red
    exit 1
}
$Vault = (Resolve-Path -LiteralPath $Vault).ProviderPath

. (Join-Path $PSScriptRoot 'common.ps1')

if (-not (Test-Path -LiteralPath (Join-Path $Vault '.obsidian') -PathType Container)) {
    Fail "$Vault is not an Obsidian vault (no .obsidian folder). Give your vault folder, not the project folder."
}

Update-Path
if (-not (Test-Path -LiteralPath (Join-Path $Root 'vantage.exe'))) {
    Fail "vantage.exe is missing. Run .\build.cmd first."
}

# The sample vault maps Atlas and Compass onto testdata\repos. Create those
# repositories the first time, so .\start.cmd testdata\vault works on a fresh tree.
function Ensure-Fixtures {
    $map = Join-Path $Vault '.vantage\repos'
    if (-not (Test-Path -LiteralPath $map)) { return }
    $alpha = Join-Path $Root 'testdata\repos\alpha\.git'
    $beta = Join-Path $Root 'testdata\repos\beta\.git'
    if ((Test-Path -LiteralPath $alpha) -and (Test-Path -LiteralPath $beta)) { return }
    $text = Get-Content -LiteralPath $map -Raw
    if ($text -notmatch 'repos/alpha' -and $text -notmatch 'repos/beta') { return }
    Require 'go'
    Step 'Generating the sample Git repositories'
    Run 'Fixture repositories' { go run ./scripts/fixtures/make-repos.go }
}

New-Item -ItemType Directory -Force -Path (Split-Path $memo) | Out-Null
Set-Content -LiteralPath $memo -Value $Vault -Encoding UTF8

Ensure-Fixtures

$url = "http://127.0.0.1:$Port"
Step "Importing vault $Vault"
if ($Repos) {
    Run "The import" { & '.\vantage.exe' import -vault $Vault -repos $Repos }
} else {
    Run "The import" { & '.\vantage.exe' import -vault $Vault }
}

if ($global:VantageOutput | Where-Object { $_ -match '^0 repositories' }) {
    Warn "no Git repository imported. Check the ""warning"" lines above if there are any."
    Info "A project has commits when its Projects\<Name>\<Name>.md card has ""type: project"""
    Info "and ""source_path:"" is a Git repository on this PC."
    Info "Or pass -Repos ""Name=C:\path"", or add <vault>\.vantage\repos."
}

Step "Starting the UI: $url  (Ctrl+C to stop)"
Info 'Live updates: an edited note or commit is reimported within a few seconds.'
Start-Job -ScriptBlock { param($Address) Start-Sleep -Seconds 2; Start-Process $Address } -ArgumentList $url | Out-Null
if ($Repos) {
    & '.\vantage.exe' watch -vault $Vault -port "$Port" -repos $Repos
} else {
    & '.\vantage.exe' watch -vault $Vault -port "$Port"
}
exit $LASTEXITCODE
