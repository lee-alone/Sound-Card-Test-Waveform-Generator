$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$root = Split-Path -Parent $PSScriptRoot
$binDir = Join-Path $root "bin"
$output = Join-Path $binDir "sound-signal-generator.exe"
$legacyCli = Join-Path $binDir "cli-mvp.exe"

New-Item -ItemType Directory -Path $binDir -Force | Out-Null
Push-Location $root
try {
    # One executable contains both the GUI and the CLI entry points.
    go build -trimpath -buildvcs=true -o $output ./cmd/app-giu
    if ($LASTEXITCODE -ne 0) {
        throw "Unified build failed with exit code $LASTEXITCODE"
    }

    # Remove the old split CLI binary so bin/ contains a single executable.
    if (Test-Path -LiteralPath $legacyCli) {
        Remove-Item -LiteralPath $legacyCli -Force
    }
}
finally {
    Pop-Location
}