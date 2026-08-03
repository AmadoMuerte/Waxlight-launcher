<div align="center">
  <img src="packaging/linux/com.waxlight.launcher.svg" alt="Waxlight Launcher icon" width="112" height="112">

# Waxlight Launcher

**A warm, focused launcher for Vintage Story.**

**English** | [Русский](README.ru.md)

[![CI](https://github.com/AmadoMuerte/Waxlight-launcher/actions/workflows/ci.yml/badge.svg)](https://github.com/AmadoMuerte/Waxlight-launcher/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/AmadoMuerte/Waxlight-launcher)](https://github.com/AmadoMuerte/Waxlight-launcher/releases/latest)
[![License: GPL-3.0](https://img.shields.io/badge/license-GPL--3.0-blue.svg)](LICENSE)

</div>

Waxlight Launcher was created and is developed solely by
[AmadoMuerte](https://github.com/AmadoMuerte). It is his personal independent
project. Its development has involved no contact, collaboration, approval, or
other relationship with the developers of Vintage Story.

Waxlight puts accounts, game versions, isolated game setups, mods, launches,
and playtime in one desktop application. It is built with Go, Wails, React,
TypeScript, and SQLite, and supports Windows and Linux.

Waxlight does not distribute the game or bypass its licensing. A valid Vintage
Story account and the right to download the game are required.

## What Waxlight does

* Manages multiple Vintage Story accounts, including TOTP/2FA login.
* Discovers releases from the Vintage Story version feed and installs several
  game versions side by side.
* Creates isolated game setups so mods and settings do not leak between them.
* Browses Vintage Story ModDB with search, filters, sorting, and mod details.
* Downloads, installs, updates, enables, disables, and removes mods per setup.
* Validates the selected account and game version before launch.
* Starts and stops the game, records logs, and prevents conflicting launches.
* Tracks individual play sessions and total or per-setup playtime.
* Shows current and completed downloads and other long-running operations.
* Supports multiple interface languages with a persistent language preference.

## Download

Download the newest build from
[GitHub Releases](https://github.com/AmadoMuerte/Waxlight-launcher/releases/latest).

| Platform            | File                            | Recommended for                                         |
| ------------------- | ------------------------------- | ------------------------------------------------------- |
| Windows x64         | `*-windows-amd64-installer.exe` | Normal installation with shortcuts and an uninstaller   |
| Windows x64         | `*-windows-amd64-portable.zip`  | Portable use without installation                       |
| Debian / Ubuntu x64 | `*-linux-amd64.deb`             | Debian, Ubuntu, Mint, and compatible distributions      |
| Fedora / RPM x64    | `*-linux-amd64.rpm`             | Fedora and compatible RPM distributions                 |
| Linux x64           | `*-linux-amd64.tar.gz`          | Other distributions with the required runtime libraries |

Every release also contains `SHA256SUMS`. Verify a download on Linux with:

```bash
sha256sum --check SHA256SUMS --ignore-missing
```

### Windows installation

Download and run the installer. Alternatively, extract the portable ZIP and
start `waxlight.exe`. Waxlight needs the Microsoft Edge WebView2 Runtime, which
is included with current Windows installations and can be installed by the
Waxlight installer when missing.

Early unsigned builds may trigger a Microsoft Defender SmartScreen warning.
Check that the file came from this repository's Releases page and verify its
checksum before choosing to run it.

### Debian and Ubuntu installation

```bash
sudo apt install ./Waxlight-Launcher-v0.1.2-linux-amd64.deb
```

### Fedora installation

```bash
sudo dnf install ./Waxlight-Launcher-v0.1.2-linux-amd64.rpm
```

### Portable Linux installation

The portable build requires GTK 3 and WebKitGTK 4.1 at runtime. Install those
packages using your distribution's package manager, then extract and run it:

```bash
tar -xzf Waxlight-Launcher-v0.1.2-linux-amd64.tar.gz
cd Waxlight-Launcher-v0.1.2-linux-amd64
./waxlight
```

An AppImage is not published yet. The portable archive is the distribution-
neutral option for the first release.

## First launch

1. Open **Accounts** and sign in with a Vintage Story account.
2. Open **Game Versions** and install a supported version.
3. Create a setup from **Library** and select its game version and account.
4. Optionally install mods from **Mods**.
5. Select the setup and press **Play**.

Waxlight stores its local database, downloaded files, and settings in the
operating system's user configuration directory:

* Linux: `~/.config/waxlight/`
* Windows: `%AppData%\waxlight\`

Stop all games before backing up or moving this directory. Never attach the
entire data directory, instance `clientsettings.json` files, or logs to a public
issue. Persistent session credentials are held by the operating-system
credential store and are not transferred with the Waxlight data directory.

## Interface languages

Waxlight currently includes:

* English;
* Russian.

The selected language is stored in the application settings and restored the
next time Waxlight starts.

Translation resources are located in:

```text
frontend/src/i18n/locales/
```

Each language uses one JSON file with stable `snake_case` keys:

```text
en.json
ru.json
```

English is the canonical source and fallback language. Contributors are welcome
to improve existing translations or add new ones.

When editing translations:

* translate values only;
* do not rename existing keys;
* preserve interpolation expressions such as `{{name}}`;
* preserve plural key suffixes such as `_one`, `_few`, `_many`, and `_other`;
* keep technical names, paths, URLs and version identifiers unchanged.

Validate translation files with:

```bash
npm run check:i18n --prefix frontend
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the complete contribution process.

## Project status

Waxlight is under active development. Version `0.1.x` should be treated as an
early release: back up important saves, review compatibility before installing
mods, and expect the UI and local data model to evolve.

Authentication is isolated behind a Go backend client. Passwords and TOTP codes
are never persisted. Persistent session keys and signatures use Secret Service
on Linux and Windows Credential Manager on Windows; production has no plaintext
fallback. React receives allow-listed account DTOs and cannot retrieve raw
session credentials through the Wails API.

For a game launch, the four fields required by Vintage Story are written to that
instance's `clientsettings.json` only after session validation. Waxlight removes
them after normal exit and launch failure, and reconciles stale fields at the
next startup after a crash. Removing an account clears affected instance
settings, but local deletion cannot be claimed to revoke an already issued
server session. See [the authentication notes](docs/authentication.md) and
[security policy](SECURITY.md) for the precise model and limitations.

Game downloads support progress, cancellation, resume where possible, and
checksum validation. See [the game-version notes](docs/game-versions.md) for
implementation details.

## Build from source

### Requirements

* Go 1.24 or newer;
* Node.js 22 and npm;
* Wails CLI 2.11;
* a C compiler;
* the [Wails platform dependencies](https://wails.io/docs/gettingstarted/installation/)
  for your operating system;
* on Linux, GTK 3 and WebKitGTK 4.1 development packages.

Clone and verify the project:

```bash
git clone https://github.com/AmadoMuerte/Waxlight-launcher.git
cd Waxlight-launcher
npm ci --include=dev --prefix frontend
go install golang.org/x/vuln/cmd/govulncheck@v1.6.0
make release-check
```

Run the desktop application with live reload:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0
cd cmd/waxlight
wails dev
```

Build a production desktop binary:

```bash
make wails-build
```

The Wails command must run from `cmd/waxlight`; the root `Makefile` provides
shortcuts for common tasks. A plain `go build` without Waxlight's desktop tags
does not produce the supported GUI build.

Useful commands:

```bash
make test                 # Go and frontend tests
make vet                  # Go static analysis
make frontend             # TypeScript and Vite production build
make package-linux        # Local .deb, .rpm, and portable archive
make release-check        # Full pre-release validation
make security             # Prohibited-pattern and vulnerability checks
```

## Architecture

The Go application follows a layered structure:

```text
Presentation (Wails) -> Application -> Domain
Infrastructure -------> Application / Domain
```

The React frontend talks to generated Wails bindings through a shared API
layer. Business rules, downloads, filesystem changes, authentication, process
management, and playtime calculation remain in Go rather than UI components.

Important directories:

```text
cmd/waxlight/             Desktop entry point and Wails configuration
internal/domain/          Core models and domain errors
internal/application/     Use cases and service interfaces
internal/infrastructure/  SQLite, HTTP, filesystem, credentials, processes
internal/presentation/    Wails controllers and DTOs
frontend/src/             React and TypeScript interface
frontend/src/i18n/        Interface localization
packaging/                Linux desktop and package metadata
scripts/                  Reproducible release scripts
.github/workflows/        CI and release automation
```

## Contributing

Contributions are welcome. Before opening a pull request:

1. Read [CONTRIBUTING.md](CONTRIBUTING.md).
2. Create a focused branch from `main`.
3. Keep business logic outside React components and Wails controllers.
4. Add or update tests for behavior changes.
5. Run `make release-check`.
6. Explain the user-facing change and testing performed in the pull request.

Use the issue templates for reproducible bugs and focused feature proposals.
For vulnerabilities or accidental credential exposure, follow
[SECURITY.md](SECURITY.md) instead of opening a public issue.

Translation contributions are also welcome. Keep translation keys synchronized
with `frontend/src/i18n/locales/en.json` and run the i18n validation command
before submitting a pull request.

## Releases

Every push and pull request to `main` runs tests, static analysis, the frontend
build, native credential-store integration tests on Linux and Windows,
vulnerability and secret scans, and a Linux Wails build.

Pushing a semantic version tag such as `v0.1.0` starts the release workflow,
which:

1. validates the tag against the application version;
2. builds Windows and Linux artifacts on native GitHub runners;
3. validates the generated packages;
4. creates `SHA256SUMS`;
5. publishes a GitHub Release with generated release notes.

Workflow actions are pinned to immutable commits and release assets receive
SHA-256 checksums. Release signing is not currently configured; published
checksums provide integrity verification after obtaining `SHA256SUMS`, but do
not replace an authenticated signing chain.

Before tagging, update the version in both Wails configuration files. Existing
release tags must never be moved.

## Author

Waxlight Launcher is designed and developed by
[AmadoMuerte](https://github.com/AmadoMuerte).

## License

Waxlight Launcher is free software licensed under the
[GNU General Public License v3.0](LICENSE). See [NOTICE](NOTICE) for third-party
and project notices.
