# Construit l'interface (web\dist) puis observatoire.exe, qui l'embarque.
# Si l'« Accès contrôlé aux dossiers » de Windows Defender est actif, autorise observatoire.exe
# (une fenêtre administrateur s'ouvre).
# Usage : .\construire.cmd

. (Join-Path $PSScriptRoot 'commun.ps1')

Actualiser-Path
Exiger 'go'
Exiger 'npm'
if (-not (Test-Path -LiteralPath 'web\node_modules')) {
    Echec "Dépendances du front absentes. Lance d'abord : .\installer.cmd"
}

Etape "Construction de l'interface (npm run build)"
Lancer "La construction de l'interface" { npm --prefix web run build }
Info "(l'avertissement « Some chunks are larger than 500 kB » est normal)"

Etape 'Compilation de observatoire.exe'
Lancer 'La compilation' { go build -tags embed -o observatoire.exe ./cmd/observatoire }

$exe = Join-Path $Racine 'observatoire.exe'
$preference = $null
try { $preference = Get-MpPreference -ErrorAction Stop } catch {}
if ($preference -and $preference.EnableControlledFolderAccess -eq 1) {
    # Sans droits admin, la liste des applications autorisées est illisible : on teste
    # directement si observatoire.exe peut écrire ici (création d'une base jetable).
    $essai = Join-Path $Racine '.essai-ecriture.db'
    $ok = $false
    try {
        & $exe migrate -db $essai *> $null
        $ok = ($LASTEXITCODE -eq 0) -and (Test-Path -LiteralPath $essai)
    } catch {}
    Remove-Item -Path "$essai*" -Force -ErrorAction SilentlyContinue
    if ($ok) {
        Info 'Accès contrôlé aux dossiers : observatoire.exe est déjà autorisé.'
    } else {
        Etape 'Accès contrôlé aux dossiers actif : autorisation de observatoire.exe (accepte la fenêtre administrateur)'
        $commande = "Add-MpPreference -ControlledFolderAccessAllowedApplications '" + ($exe -replace "'", "''") + "'"
        $encodee = [Convert]::ToBase64String([System.Text.Encoding]::Unicode.GetBytes($commande))
        try {
            $p = Start-Process powershell -Verb RunAs -Wait -PassThru -ArgumentList "-NoProfile -EncodedCommand $encodee"
            if ($p.ExitCode -ne 0) { throw "code $($p.ExitCode)" }
            Info 'observatoire.exe autorisé.'
        } catch {
            Attention "autorisation non faite ($($_.Exception.Message))."
            Info "Sans elle, observatoire.exe ne pourra pas créer sa base. À faire à la main : Sécurité Windows >"
            Info "Protection contre les virus et menaces > Gérer la protection contre les ransomware >"
            Info "Autoriser une application via l'Accès contrôlé aux dossiers > $exe"
        }
    }
}

Write-Host ''
Write-Host 'Build terminé : observatoire.exe' -ForegroundColor Green
Write-Host 'Étape suivante : .\lancer.cmd "C:\chemin\vers\ton coffre"'
