<div align="center">
  <img src="./docs/waxlight.png" alt="Waxlight Launcher" width="180">

# Waxlight Launcher 

**A modern, lightweight launcher for Vintage Story.**

**English** · [Русский](docs/README.ru.md)

[![CI](https://github.com/AmadoMuerte/Waxlight-launcher/actions/workflows/ci.yml/badge.svg)](https://github.com/AmadoMuerte/Waxlight-launcher/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/AmadoMuerte/Waxlight-launcher)](https://github.com/AmadoMuerte/Waxlight-launcher/releases/latest)
[![License: GPL-3.0](https://img.shields.io/badge/license-GPL--3.0-blue.svg)](LICENSE)
[![Support development](https://img.shields.io/badge/Support-Development-8A2BE2)](https://hipolink.net/amadomuerte)

[Download](https://github.com/AmadoMuerte/Waxlight-launcher/releases/latest) · [Guide](https://amadomuerte.github.io/Waxlight-launcher/) · [Discord](https://discord.gg/CrRHvg9UVw) · [Privacy Policy](docs/PRIVACY.md) · [Code Signing Policy](docs/CODE_SIGNING_POLICY.md) · [Issues](https://github.com/AmadoMuerte/Waxlight-launcher/issues) · [Support](https://hipolink.net/amadomuerte)
</div>
 
Waxlight is an independent, open-source launcher that brings Vintage Story accounts, game versions, isolated instances, mods, updates, and playtime into one desktop app for **Windows and Linux**.

It is maintained by [AmadoMuerte](https://github.com/AmadoMuerte) with help from contributors. Waxlight is not affiliated with or endorsed by the developers of Vintage Story and does not distribute the game or bypass its licensing.

## Features 

- Multiple Vintage Story accounts with TOTP/2FA support.
- Multiple game versions installed side by side.
- Isolated instances with separate mods and settings.
- Built-in Vintage Story ModDB browser with search, filters, and mod management.
- Mod installation, updates, enable/disable, and removal per instance.
- Playtime tracking, launch logs, downloads, and background operations.
- Stable and prerelease launcher updates with checksum verification.
- Interface languages: English, Русский, Беларуская, Español, Français, Deutsch, Қазақша, Polski, Svenska, and Português.
- Custom data folder for versions, instances, mods, downloads, and the database.

## Download

Get the latest version from [GitHub Releases](https://github.com/AmadoMuerte/Waxlight-launcher/releases/latest).

| Platform | Package |
| --- | --- |
| Windows x64 | Installer `.exe` or portable `.zip` |
| Debian / Ubuntu x64 | `.deb` |
| Fedora / RPM x64 | `.rpm` |
| Other Linux x64 | Portable `.tar.gz` |

Each release includes `SHA256SUMS` for integrity checks.

> On Windows, unsigned Waxlight builds may trigger Microsoft Defender SmartScreen. Download Waxlight only from this repository's Releases page.

## Getting started

1. Sign in under **Accounts**.
2. Install a game version under **Game Versions**.
3. Create an instance in **Library** and select its account and game version.
4. Install mods from **Mods** if needed.
5. Press **Play**.

A valid Vintage Story account with access to the game is required.

## Data & privacy

Default data locations:

- Linux: `~/.config/waxlight/`
- Windows: `%AppData%\waxlight\`

The main data folder can be moved from **Settings → Data folder**. Account credentials remain in the operating system credential store.

Optional telemetry is disabled by default for new installations. When enabled, it sends a pseudonymous installation ID, launcher version, OS, architecture, and limited numeric or allowlisted operational data. It can be changed in **Settings → Privacy & telemetry**. See the [Privacy Policy](docs/PRIVACY.md) for the complete, current list of network transfers.

See [SECURITY.md](docs/SECURITY.md) and [authentication notes](docs/authentication.md) for details.

## Build from source

Requirements: **Go 1.25+**, **Node.js 22+**, **Wails 2.11**, a C compiler, and the required [Wails platform dependencies](https://wails.io/docs/gettingstarted/installation/).

```bash
git clone https://github.com/AmadoMuerte/Waxlight-launcher.git
cd Waxlight-launcher
npm ci --include=dev --prefix frontend
go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0
cd cmd/waxlight
wails dev
```

Production build from the repository root:

```bash
make wails-build
```

## Contributing

Code, translations, testing, documentation, bug reports, and focused feature proposals are welcome.

Before opening a pull request, read [CONTRIBUTING.md](docs/CONTRIBUTING.md) and run:

```bash
make release-check
```

For security issues, follow [SECURITY.md](docs/SECURITY.md) instead of opening a public issue.

## Contributors

<a href="https://github.com/AmadoMuerte/Waxlight-launcher/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=AmadoMuerte/Waxlight-launcher" alt="Waxlight Launcher contributors">
</a>

## Support development

Waxlight is free and open source. If you enjoy the project and want to support its continued development:

[![Support development](https://img.shields.io/badge/Support-Development-8A2BE2?style=for-the-badge)](https://hipolink.net/amadomuerte)

## License

Waxlight Launcher is licensed under the [GNU General Public License v3.0](LICENSE). See [NOTICE](NOTICE) for third-party and project notices.
