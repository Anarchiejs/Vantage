# Build the UI (web\dist) then observatoire.exe, which embeds it.
# If Windows Defender Controlled folder access is on, allow observatoire.exe
# (an administrator window opens).
# Usage: .\build.cmd

. (Join-Path $PSScriptRoot 'common.ps1')

Update-Path
Require 'go'
Require 'npm'
if (-not (Test-Path -LiteralPath 'web\node_modules')) {
    Fail "UI dependencies are missing. Run .\install.cmd first."
}

Step "Building the UI (npm run build)"
Run "The UI build" { npm --prefix web run build }

Step 'Compiling observatoire.exe'
Run 'The compile' { go build -tags embed -o observatoire.exe ./cmd/observatoire }

$exe = Join-Path $Root 'observatoire.exe'
$preference = $null
try { $preference = Get-MpPreference -ErrorAction Stop } catch {}
if ($preference -and $preference.EnableControlledFolderAccess -eq 1) {
    # Without admin rights the allow-list cannot be read: test directly
    # whether observatoire.exe can write here (create a throwaway database).
    $probe = Join-Path $Root '.write-probe.db'
    $ok = $false
    try {
        & $exe migrate -db $probe *> $null
        $ok = ($LASTEXITCODE -eq 0) -and (Test-Path -LiteralPath $probe)
    } catch {}
    Remove-Item -Path "$probe*" -Force -ErrorAction SilentlyContinue
    if ($ok) {
        Info 'Controlled folder access: observatoire.exe is already allowed.'
    } else {
        Step 'Controlled folder access is on: allowing observatoire.exe (accept the administrator window)'
        $command = "Add-MpPreference -ControlledFolderAccessAllowedApplications '" + ($exe -replace "'", "''") + "'"
        $encoded = [Convert]::ToBase64String([System.Text.Encoding]::Unicode.GetBytes($command))
        try {
            $p = Start-Process powershell -Verb RunAs -Wait -PassThru -ArgumentList "-NoProfile -EncodedCommand $encoded"
            if ($p.ExitCode -ne 0) { throw "code $($p.ExitCode)" }
            Info 'observatoire.exe allowed.'
        } catch {
            Warn "permission was not granted ($($_.Exception.Message))."
            Info "Without it, observatoire.exe cannot create its database. Do it by hand: Windows Security >"
            Info "Virus & threat protection > Manage ransomware protection >"
            Info "Allow an app through Controlled folder access > $exe"
        }
    }
}

Write-Host ''
Write-Host 'Build complete: observatoire.exe' -ForegroundColor Green
Write-Host 'Next step: .\start.cmd "C:\path\to\your vault"'
