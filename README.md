# igc-sync

A cross-platform CLI tool that continuously syncs IGC flight files from [gliding.net.nz](https://gliding.net.nz) to a local folder. Designed for event organisers running SeeYou live scoring.

Relates to: [glidingnz/58gliding.net.nz#231](https://github.com/glidingnz/58gliding.net.nz/issues/231)

## Download

Pre-built binaries for Windows, macOS, and Linux are attached to each [GitHub Release](../../releases).

## Usage

```sh
# Run from the folder where you want IGC files to land
cd /path/to/seeyou/watch/folder
igc-sync
```

1. A loading screen fetches the event list from gliding.net.nz.
2. Select an event using the arrow keys and press Enter.
3. Files are downloaded to `./{event-slug}/{filename}.igc` and kept in sync, polling every 60 seconds.
4. Press `q` or `Ctrl+C` to stop.

No authentication is required — the gliding.net.nz API is public.

## Development

**Requirements:** Go 1.22+

```sh
# Clone and run tests
git clone https://github.com/glidingnz/igc-sync
cd igc-sync
go test ./...

# Live tests: real anonymous GETs against gliding.net.nz (CI runs these too)
go test -tags live ./internal/live/

# Activate git hooks (blocks direct pushes to main)
git config core.hooksPath .githooks

# Run locally
go run .

# Build for your current platform
go build -o igc-sync .
```

### Cross-compilation

```sh
# Linux (amd64 / arm64)
GOOS=linux GOARCH=amd64 go build -o igc-sync-linux-amd64 .
GOOS=linux GOARCH=arm64 go build -o igc-sync-linux-arm64 .

# macOS (Intel / Apple Silicon)
GOOS=darwin GOARCH=amd64 go build -o igc-sync-darwin-amd64 .
GOOS=darwin GOARCH=arm64 go build -o igc-sync-darwin-arm64 .

# Windows
GOOS=windows GOARCH=amd64 go build -o igc-sync-windows-amd64.exe .
```

### Mock date (for testing event window)

Set `IGC_SYNC_NOW=YYYY-MM-DD` to override the current date used when selecting which events to show:

```sh
IGC_SYNC_NOW=2026-11-01 go run .
```

## Releasing

Versions follow [SemVer](https://semver.org/). For a CLI, the public API is everything an organiser's setup relies on: how it is run, where files land and what they are called, and the supported platforms.

| Bump | When |
|---|---|
| Major | An existing setup breaks: a different output folder or filenames, a dropped platform, a new required step |
| Minor | New capability; existing setups unaffected |
| Patch | Bug fixes, and Go dependency bumps worth shipping (e.g. a security fix) |
| None | Tests, CI, docs and GitHub Actions bumps; the binary doesn't change |

Weekly Dependabot bumps accumulate on `main` until there is a reason to ship. Release well before a competition, not during one: organisers download the binary once and leave it running.

To release, with CI green on `main`:

```sh
gh release create vX.Y.Z --target main --generate-notes
```

or, on GitHub: **Releases → Draft a new release**, type the new tag, target `main`, **Generate release notes**, **Publish**.

Publishing triggers the [Release workflow](.github/workflows/release.yml), which tests, builds all four platform binaries with the tag embedded, and attaches them to the release within a few minutes. If it fails, fix `main` and re-run the workflow; the release itself can stay.

## Output structure

```
{cwd}/
└── central-plateau-soaring-competition-oct-2026/
    ├── 637GBE1.igc
    ├── 637GBH1.igc
    ├── 637GDX1.igc
    ├── 637GFE1.igc
    ├── 637GHD1.igc
    ├── 637GKT1.igc
    ├── 637GKW1.igc
    ├── 637GLL1.igc
    ├── 637GML1.igc
    └── ...
```

## How it works

- Shows the 5 most recent past events and next 10 upcoming/active events; cursor starts on the closest one to now
- Polls `GET /api/v1/events/{id}/igc-files` once per minute, sending `User-Agent: igc-sync/<version>` so a WAF can allow it
- Compares the `file_hash` (SHA-256) of each remote file against the local copy
- Downloads new files and re-downloads files whose hash has changed (e.g. organiser re-uploaded a corrected file)
- Verifies SHA-256 after each download; discards the file if it doesn't match
