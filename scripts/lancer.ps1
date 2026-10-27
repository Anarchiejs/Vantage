# Importe ton coffre Obsidian puis lance l'interface (http://127.0.0.1:7878).
# Usage : .\lancer.cmd "C:\chemin\vers\ton coffre"   (la première fois)
#         .\lancer.cmd                               (ensuite : réutilise le dernier coffre)
# Ctrl+C arrête le serveur.

param(
    [Parameter(Position = 0)]
    [string]$Coffre
)

$memo = Join-Path $env:LOCALAPPDATA 'Observatoire\coffre.txt'

if (-not $Coffre) {
    if (Test-Path -LiteralPath $memo) {
        $Coffre = (Get-Content -LiteralPath $memo -TotalCount 1).Trim()
        Write-Host "Coffre mémorisé : $Coffre"
    } else {
        Write-Host ''
        Write-Host 'ERREUR : indique le dossier de ton coffre Obsidian la première fois, par exemple :' -ForegroundColor Red
        Write-Host '  .\lancer.cmd "C:\Users\toi\Documents\Mon Coffre"'
        exit 1
    }
}

# Chemin résolu avant de quitter le dossier courant : un chemin relatif reste valable.
if (-not (Test-Path -LiteralPath $Coffre -PathType Container)) {
    Write-Host ''
    Write-Host "ERREUR : dossier introuvable : $Coffre (indique le dossier de ton coffre Obsidian, entre guillemets)." -ForegroundColor Red
    exit 1
}
$Coffre = (Resolve-Path -LiteralPath $Coffre).ProviderPath

. (Join-Path $PSScriptRoot 'commun.ps1')

if (-not (Test-Path -LiteralPath (Join-Path $Coffre '.obsidian') -PathType Container)) {
    Echec "$Coffre n'est pas un coffre Obsidian (pas de dossier .obsidian). Indique le dossier de ton coffre, pas celui du projet."
}

Actualiser-Path
if (-not (Test-Path -LiteralPath (Join-Path $Racine 'observatoire.exe'))) {
    Echec "observatoire.exe est absent. Lance d'abord : .\construire.cmd"
}

New-Item -ItemType Directory -Force -Path (Split-Path $memo) | Out-Null
Set-Content -LiteralPath $memo -Value $Coffre -Encoding UTF8

Etape "Import du coffre $Coffre"
Lancer "L'import" { & '.\observatoire.exe' import -coffre $Coffre }

if ($global:ObsSortie | Where-Object { $_ -match '^0 d' }) {
    Attention "aucun dépôt Git importé. Vérifie les lignes « attention » ci-dessus s'il y en a."
    Info "Un projet n'a de commits que si sa fiche Projets\<Nom>\<Nom>.md contient « type: projet »"
    Info "et « chemin_source: » avec le chemin absolu d'un dépôt Git présent sur ce PC."
}

Etape 'Lancement de l''interface : http://127.0.0.1:7878  (Ctrl+C pour arrêter)'
Info 'Mise à jour automatique : une note ou un commit modifié est réimporté en quelques secondes.'
Start-Job -ScriptBlock { Start-Sleep -Seconds 2; Start-Process 'http://127.0.0.1:7878' } | Out-Null
& '.\observatoire.exe' watch -coffre $Coffre
exit $LASTEXITCODE
