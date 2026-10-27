# Fonctions communes aux scripts d'Observatoire (compatibles PowerShell 5.1).
# Chargé par « . (Join-Path $PSScriptRoot 'commun.ps1') » : se place à la racine du dépôt.

$ErrorActionPreference = 'Stop'
# Les programmes (go, npm, observatoire.exe) écrivent en UTF-8 : sans cela, PowerShell 5.1 abîme les accents.
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch {}
$OutputEncoding = [System.Text.Encoding]::UTF8

$Racine = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $Racine

function Etape([string]$Texte) { Write-Host "==> $Texte" -ForegroundColor Cyan }
function Info([string]$Texte) { Write-Host "    $Texte" }
function Attention([string]$Texte) { Write-Host "    Attention : $Texte" -ForegroundColor Yellow }
function Echec([string]$Texte) {
    Write-Host ''
    Write-Host "ERREUR : $Texte" -ForegroundColor Red
    exit 1
}

# Relit le PATH machine et utilisateur (utile juste après une installation par winget).
function Actualiser-Path {
    if ($env:OS -ne 'Windows_NT') { return }
    $parties = @(
        [Environment]::GetEnvironmentVariable('Path', 'Machine'),
        [Environment]::GetEnvironmentVariable('Path', 'User'),
        $env:Path
    ) | Where-Object { $_ }
    $env:Path = $parties -join ';'
}

# Lance une commande externe et s'arrête si elle échoue (code de sortie non nul).
# Sa sortie d'erreur est affichée comme du texte normal : en PowerShell 5.1, un simple avertissement
# écrit sur stderr (par exemple celui de Vite sur la taille des fichiers) deviendrait sinon une erreur.
# Les lignes affichées restent dans $global:ObsSortie.
function Lancer([string]$Description, [scriptblock]$Commande) {
    $ancien = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    $global:LASTEXITCODE = -1
    $global:ObsSortie = @()
    try {
        & $Commande 2>&1 | ForEach-Object {
            $ligne = "$_"
            $global:ObsSortie += $ligne
            Write-Host "    $ligne"
        }
        $code = $global:LASTEXITCODE
    } finally {
        $ErrorActionPreference = $ancien
    }
    if ($code -ne 0) { Echec "$Description a échoué (code $code). Lis les messages ci-dessus." }
}

function Exiger([string]$Outil) {
    if (-not (Get-Command $Outil -ErrorAction SilentlyContinue)) {
        Echec "$Outil est introuvable. Lance d'abord : .\installer.cmd (puis, si besoin, ferme et rouvre PowerShell)."
    }
}
