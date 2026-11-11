# Import your Obsidian vault, then serve the UI (http://127.0.0.1:7878).
# Usage: .\start.cmd "C:\path\to\your vault"   (the first time)
#        .\start.cmd                            (later: reuse the last vault)
# Ctrl+C stops the server.

param(
    [Parameter(Position = 0)]
    [string]$Vault
)

$memo = Join-Path $env:LOCALAPPDATA 'Observatoire\vault.txt'

if (-not $Vault) {
    if (Test-Path -LiteralPath $memo) {
        $Vault = (Get-Content -LiteralPath $memo -TotalCount 1).Trim()
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
if (-not (Test-Path -LiteralPath (Join-Path $Root 'observatoire.exe'))) {
    Fail "observatoire.exe is missing. Run .\build.cmd first."
}

New-Item -ItemType Directory -Force -Path (Split-Path $memo) | Out-Null
Set-Content -LiteralPath $memo -Value $Vault -Encoding UTF8

Step "Importing vault $Vault"
Run "The import" { & '.\observatoire.exe' import -vault $Vault }

if ($global:ObsOutput | Where-Object { $_ -match '^0 repositories' }) {
    Warn "no Git repository imported. Check the ""warning"" lines above if there are any."
    Info "A project has commits only when its Projects\<Name>\<Name>.md card contains ""type: project"""
    Info "and ""source_path:"" with the absolute path of a Git repository on this PC."
}

Step 'Starting the UI: http://127.0.0.1:7878  (Ctrl+C to stop)'
Info 'Live updates: an edited note or commit is reimported within a few seconds.'
Start-Job -ScriptBlock { Start-Sleep -Seconds 2; Start-Process 'http://127.0.0.1:7878' } | Out-Null
& '.\observatoire.exe' watch -vault $Vault
exit $LASTEXITCODE
