$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$root = Split-Path -Parent $PSScriptRoot
$binDir = Join-Path $root "bin"
$output = Join-Path $binDir "sound-signal-generator.exe"
$legacyCli = Join-Path $binDir "cli-mvp.exe"

New-Item -ItemType Directory -Path $binDir -Force | Out-Null
Push-Location $root
try {
    # Compile Windows application icon and resource into syso if windres is available
    $rcFile = Join-Path $root "assets\icon\app.rc"
    $icoFile = Join-Path $root "assets\icon\app.ico"
    $sysoFile = Join-Path $root "cmd\app-giu\resource_windows_amd64.syso"
    $windres = Get-Command "windres.exe" -ErrorAction SilentlyContinue

    if ($windres -and (Test-Path -LiteralPath $rcFile) -and (Test-Path -LiteralPath $icoFile)) {
        $needRebuild = -not (Test-Path -LiteralPath $sysoFile)
        if (-not $needRebuild) {
            $sysoTime = (Get-Item $sysoFile).LastWriteTime
            $rcTime = (Get-Item $rcFile).LastWriteTime
            $icoTime = (Get-Item $icoFile).LastWriteTime
            if ($rcTime -gt $sysoTime -or $icoTime -gt $sysoTime) {
                $needRebuild = $true
            }
        }

        if ($needRebuild) {
            Write-Host "Compiling Windows resource: $rcFile -> $sysoFile"
            & $windres.Source -O coff -F pe-x86-64 -I "$root\assets\icon" -i $rcFile -o $sysoFile
            if ($LASTEXITCODE -ne 0) {
                Write-Warning "windres failed with exit code $LASTEXITCODE; proceeding with existing syso if available."
            }
        }
    }

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