# Build the UI (web\dist) then vantage.exe, which embeds it.
# If Windows Defender Controlled folder access is on, allow vantage.exe
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

Step 'Compiling vantage.exe'
Run 'The compile' { go build -tags embed -o vantage.exe ./cmd/vantage }

$exe = Join-Path $Root 'vantage.exe'
$preference = $null
try { $preference = Get-MpPreference -ErrorAction Stop } catch {}
if ($preference -and $preference.EnableControlledFolderAccess -eq 1) {
    # Without admin rights the allow-list cannot be read: test directly
    # whether vantage.exe can write here (create a throwaway database).
    $probe = Join-Path $Root '.write-probe.db'
    $ok = $false
    try {
        & $exe migrate -db $probe *> $null
        $ok = ($LASTEXITCODE -eq 0) -and (Test-Path -LiteralPath $probe)
    } catch {}
    Remove-Item -Path "$probe*" -Force -ErrorAction SilentlyContinue
    if ($ok) {
        Info 'Controlled folder access: vantage.exe is already allowed.'
    } else {
        Step 'Controlled folder access is on: allowing vantage.exe (accept the administrator window)'
        $command = "Add-MpPreference -ControlledFolderAccessAllowedApplications '" + ($exe -replace "'", "''") + "'"
        $encoded = [Convert]::ToBase64String([System.Text.Encoding]::Unicode.GetBytes($command))
        try {
            $p = Start-Process powershell -Verb RunAs -Wait -PassThru -ArgumentList "-NoProfile -EncodedCommand $encoded"
            if ($p.ExitCode -ne 0) { throw "code $($p.ExitCode)" }
            Info 'vantage.exe allowed.'
        } catch {
            Warn "permission was not granted ($($_.Exception.Message))."
            Info "Without it, vantage.exe cannot create its database. Do it by hand: Windows Security >"
            Info "Virus & threat protection > Manage ransomware protection >"
            Info "Allow an app through Controlled folder access > $exe"
        }
    }
}

Write-Host ''
Write-Host 'Build complete: vantage.exe' -ForegroundColor Green
Write-Host 'Next step: .\start.cmd "C:\path\to\your vault"'
