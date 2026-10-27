# Installe ce qu'il faut pour construire Observatoire : git, go, node (via winget si absents),
# puis les dépendances Go et npm.
# Usage : .\installer.cmd

. (Join-Path $PSScriptRoot 'commun.ps1')

$outils = @(
    @{ Nom = 'git';  Id = 'Git.Git' },
    @{ Nom = 'go';   Id = 'GoLang.Go' },
    @{ Nom = 'node'; Id = 'OpenJS.NodeJS.LTS' }
)

Etape 'Vérification de git, go et node'
foreach ($o in $outils) {
    if (Get-Command $o.Nom -ErrorAction SilentlyContinue) {
        Info "$($o.Nom) : déjà installé"
        continue
    }
    if (-not (Get-Command winget -ErrorAction SilentlyContinue)) {
        Echec "$($o.Nom) est absent et winget est introuvable. Installe « App Installer » depuis le Microsoft Store (ou $($o.Nom) à la main), puis relance ce script."
    }
    Etape "Installation de $($o.Nom) avec winget ($($o.Id))"
    Lancer "L'installation de $($o.Nom)" { winget install --id $o.Id -e --silent --accept-package-agreements --accept-source-agreements }
    Actualiser-Path
    if (-not (Get-Command $o.Nom -ErrorAction SilentlyContinue)) {
        Echec "$($o.Nom) vient d'être installé mais reste introuvable : ferme et rouvre PowerShell, puis relance ce script."
    }
    Info "$($o.Nom) : installé"
}
Exiger 'npm'

$versionNode = ("$(& node --version)").Trim().TrimStart('v')
$majeure = 0
[void][int]::TryParse($versionNode.Split('.')[0], [ref]$majeure)
if ($majeure -lt 22) {
    Attention "Node $versionNode détecté ; les tests du front demandent Node 22.12 ou plus (winget upgrade OpenJS.NodeJS.LTS)."
}

Etape 'Dépendances Go (go mod download)'
Lancer 'go mod download' { go mod download }

if (Test-Path -LiteralPath 'web\package-lock.json') {
    Etape 'Dépendances du front (npm ci)'
    Lancer 'npm ci' { npm --prefix web ci }
} else {
    Etape 'Dépendances du front (npm install)'
    Lancer 'npm install' { npm --prefix web install }
}

Write-Host ''
Write-Host 'Installation terminée.' -ForegroundColor Green
Write-Host 'Étape suivante : .\construire.cmd'
