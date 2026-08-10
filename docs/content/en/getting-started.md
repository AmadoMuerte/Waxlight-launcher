---
title: Getting started
description: Installing Waxlight Launcher and first steps — from download to launching the game in minutes.
order: 10
---

# Getting started

From download to launching the game in a few minutes.

## Download

Official builds are published **only** on the [GitHub Releases](https://github.com/AmadoMuerte/Waxlight-launcher/releases/latest) page. Every release includes a `SHA256SUMS` file for integrity verification.

| Platform | Package | File |
| --- | --- | --- |
| Windows x64 | Installer (shortcuts and uninstaller included) | `*-windows-amd64-installer.exe` |
| Windows x64 | Portable (no installation) | `*-windows-amd64-portable.zip` |
| Debian / Ubuntu x64 | .deb package (Debian, Ubuntu, Mint, and compatible) | `*-linux-amd64.deb` |
| Fedora / RPM x64 | .rpm package (Fedora and RPM-compatible) | `*-linux-amd64.rpm` |
| Other Linux x64 | Portable archive (requires GTK 3 and WebKitGTK 4.1) | `*-linux-amd64.tar.gz` |

> [!WARNING] Windows SmartScreen
> Early unsigned builds may trigger a Microsoft Defender SmartScreen warning. Download Waxlight only from the official repository's Releases page. See [Code signing](./policies/code-signing.md) for details.

## Integrity check

Compare the downloaded file's SHA-256 with the value from `SHA256SUMS`:

```bash
# Linux
sha256sum -c SHA256SUMS --ignore-missing

# Windows (PowerShell)
Get-FileHash .\waxlight-*-windows-amd64-installer.exe -Algorithm SHA256
```

SHA-256 confirms the file is byte-identical to the release artifact but does **not** authenticate the publisher. That is what the Authenticode signature is for — see the [code signing policy](./policies/code-signing.md).

## First steps

1. **Sign in.** Open Accounts and sign in with a Vintage Story account. TOTP/2FA is supported. See [Accounts & sign-in](./features/accounts.md).
2. **Install a game version.** Go to Game Versions → pick a version → Download. Progress is visible on the [Operations](./features/operations.md) page.
3. **Create an instance.** In Library, create an instance and select its account and game version.
4. **Install mods (optional).** The Mods section is a built-in Vintage Story ModDB browser with search and filters.
5. **Press Play.** A valid Vintage Story account with access to the game is required.

## Where data is stored

| Platform | Default directory |
| --- | --- |
| Linux | `~/.config/waxlight/` |
| Windows | `%AppData%\waxlight\` |

Inside are game versions (`versions`), instances (`instances`), downloads (`downloads`), cache (`cache`), backups (`backups`), logs (`logs`), and the launcher database. The main data folder can be relocated: **Settings → Data folder**. Account credentials stay in the OS credential store regardless.

## Telemetry on first run

Optional telemetry is **disabled by default**. The Windows installer shows the privacy policy with a separate, unchecked opt-in box. The setting can be changed at any time: **Settings → Privacy & telemetry**. The full list of transmitted fields is in the [Privacy Policy](./policies/privacy.md).

## Need help?

Check the [FAQ](./faq.md), ask on the [Discord server](https://discord.gg/CrRHvg9UVw), or open an [issue on GitHub](https://github.com/AmadoMuerte/Waxlight-launcher/issues).
