$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$root = Split-Path -Parent $PSScriptRoot
$binDir = Join-Path $root "bin"
New-Item -ItemType Directory -Path $binDir -Force | Out-Null
Push-Location $root
try {
    go build -trimpath -buildvcs=true -o (Join-Path $binDir "sound-signal-generator.exe") ./cmd/app-giu
    if ($LASTEXITCODE -ne 0) {
        throw "GIU build failed with exit code $LASTEXITCODE"
    }

    go build -trimpath -buildvcs=true -o (Join-Path $binDir "cli-mvp.exe") ./cmd/cli-mvp
    if ($LASTEXITCODE -ne 0) {
        throw "CLI build failed with exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}
