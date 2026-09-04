<div align="center">
  <img src="./docs/waxlight.png" alt="Waxlight Launcher" width="180">

# Waxlight Launcher 

**A modern, lightweight launcher for Vintage Story.**

**English** · [Русский](docs/README.ru.md)

[![CI](https://github.com/AmadoMuerte/Waxlight-launcher/actions/workflows/ci.yml/badge.svg)](https://github.com/AmadoMuerte/Waxlight-launcher/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/AmadoMuerte/Waxlight-launcher)](https://github.com/AmadoMuerte/Waxlight-launcher/releases/latest)
[![License: GPL-3.0](https://img.shields.io/badge/license-GPL--3.0-blue.svg)](LICENSE)
[![Support development](https://img.shields.io/badge/Support-Development-8A2BE2)](https://hipolink.net/amadomuerte)

[Download](https://github.com/AmadoMuerte/Waxlight-launcher/releases/latest) · [Discord](https://discord.gg/CrRHvg9UVw) · [Privacy Policy](docs/PRIVACY.md) · [Code Signing Policy](docs/CODE_SIGNING_POLICY.md) · [Issues](https://github.com/AmadoMuerte/Waxlight-launcher/issues) · [Support](https://hipolink.net/amadomuerte)
</div>
 
Waxlight is an independent, open-source launcher that brings Vintage Story accounts, game versions, isolated instances, mods, updates, and playtime into one desktop app for **Windows and Linux**.

It is maintained by [AmadoMuerte](https://github.com/AmadoMuerte) with help from contributors. Waxlight is not affiliated with or endorsed by the developers of Vintage Story and does not distribute the game or bypass its licensing.

## Features 

- **Instance management** — isolated instances, cloning, existing-install import, `.waxlight` import/export, custom covers, and per-instance launch settings.
- **Game versions** — install and keep multiple Vintage Story versions side by side.
- **Mods** — browse ModDB, install local mods, choose versions, manage dependencies and update policies, and update per instance.
- **Backups and recovery** — manual and automatic snapshots, configurable retention, and Last Known Good recovery.
- **Accounts and clients** — multiple Vintage Story accounts and Optimum support.
- **Servers and sharing** — public server browser, favorites, and deep links for shareable mod and server pages.
- **Activity** — playtime statistics, launch logs, downloads, and background operation history.
- **News and updates** — official Vintage Story news plus stable and prerelease launcher updates with checksum verification.
- **Desktop support** — native Windows and Linux packages with a movable launcher data folder.
- **Community translations** — English, Russian, German, French, Spanish, Portuguese and many other community translations.

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

## NixOS

Waxlight is packaged as a [Nix flake](https://nixos.wiki/wiki/Flakes) for `x86_64-linux`. The
package builds the launcher natively against GTK3 and WebKitGTK 4.1 — no `steam-run`, `nix-ld`,
or manual library setup is needed. All user data (configuration, instances, mods, logs, and
account sessions) stays in the standard XDG directories under `~/.config/waxlight/` and is
never written into the immutable Nix store.

Run Waxlight without installing:

```bash
nix run github:AmadoMuerte/Waxlight-launcher
```

Install it into your user profile, which also adds it to your desktop application launcher:

```bash
nix profile install github:AmadoMuerte/Waxlight-launcher
```

To use Waxlight from another flake, for example as part of your NixOS configuration
(`flake.nix`):

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    waxlight.url = "github:AmadoMuerte/Waxlight-launcher";
  };

  outputs = { nixpkgs, waxlight, ... }: {
    nixosConfigurations.my-machine = nixpkgs.lib.nixosSystem {
      modules = [
        ({ pkgs, ... }: {
          environment.systemPackages = [ waxlight.packages.${pkgs.system}.default ];
        })
      ];
    };
  };
}
```

Uninstall Waxlight from the user profile:

```bash
nix profile remove Waxlight-launcher
```

Launcher self-updates are disabled in the Nix build: Waxlight never tries to replace its own
executable inside `/nix/store`. Update it through Nix instead:

```bash
nix profile upgrade Waxlight-launcher
```

## Getting started

1. Sign in under **Accounts**.
2. Install a game version under **Game Versions**.
3. Create an instance in **Library** and select its account and game version.
4. Install mods from **Mods** if needed.
5. Press **Play**.

A valid Vintage Story account with access to the game is required.

> On Windows, the game also requires the **Microsoft Visual C++ Redistributable 2015–2022 (x64)**. Without it the game may fail to start with an `Unable to load DLL 'nanosvg' (or one of its dependencies)` error. Install it from <https://aka.ms/vs/17/release/vc_redist.x64.exe>.

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

## Documentation

The backend API reference is generated from the Wails transport source and GoDoc with the pinned WailsDoc dependency:

```bash
make api-docs       # generate schema, Markdown, and the checked API inventory
make api-docs-dev   # generate and start the VitePress development server
make api-docs-build # generate and build the production VitePress site
```

Generated schema, Markdown, and VitePress output are intentionally not committed. A clean checkout reproduces them from `internal/transport/wails`, `wailsdoc.yaml`, and `docs/wails-api-inventory.json` remains the checked public API contract.

Contributors can browse the current backend API reference at <https://docs.waxlight.by>.

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
