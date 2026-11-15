# Legacy Wails UI

This directory tree contains the historical Wails/Vue test interface. It is not
part of the production application, the default test suite, or the release
build.

## Status

- Frozen development/test harness.
- No new product features should be added here.
- The supported desktop application is `cmd/app-giu`.
- The supported automation/probe entry point is `cmd/cli-mvp`.

## Shared Code

The following packages are shared product code and must not be moved or removed
with the Wails UI:

- `internal/controller`
- `internal/audio`
- `internal/engine`
- `internal/dsp`
- `internal/dsd`

Wails-specific code is limited to `cmd/app`, `frontend` and the embedded
`cmd/app/dist` assets.

## Default Build

PowerShell:

```powershell
.\scripts\build.ps1
```

This builds only:

- `bin/sound-signal-generator.exe` from `cmd/app-giu`
- `bin/cli-mvp.exe`

## Explicit Legacy Build

```powershell
.\scripts\build.ps1 -LegacyWails
```

or manually:

```powershell
go build -trimpath -buildvcs=true -tags "legacy_wails,desktop,production" `
  -o bin/wails-legacy.exe ./cmd/app
```

The Wails frontend must be rebuilt separately when changing Vue sources:

```powershell
Push-Location frontend
npm run build -- --outDir ../cmd/app/dist --emptyOutDir
Pop-Location
```

## Future Disposition

Keep the source in the main repository for at least one release cycle. If it is
still unused after that, move it to a separate legacy module or archive branch.
A Git tag should be created from a committed tree that includes the final Wails
source; tags do not capture uncommitted working-tree changes.
