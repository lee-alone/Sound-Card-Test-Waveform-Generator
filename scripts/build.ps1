param(
    [switch]$LegacyWails
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$root = Split-Path -Parent $PSScriptRoot
$binDir = Join-Path $root "bin"
New-Item -ItemType Directory -Path $binDir -Force | Out-Null
Push-Location $root
try {
    if ($LegacyWails) {
        Push-Location (Join-Path $root "frontend")
        try {
            npm run build -- --outDir ../cmd/app/dist --emptyOutDir
            if ($LASTEXITCODE -ne 0) {
                throw "Legacy Wails frontend build failed with exit code $LASTEXITCODE"
            }
        }
        finally {
            Pop-Location
        }

        go build -trimpath -buildvcs=true -tags "legacy_wails,desktop,production" `
            -o (Join-Path $binDir "wails-legacy.exe") ./cmd/app
        if ($LASTEXITCODE -ne 0) {
            throw "Legacy Wails build failed with exit code $LASTEXITCODE"
        }
        return
    }

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
