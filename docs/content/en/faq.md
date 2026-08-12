---
title: FAQ
description: Frequently asked questions about installation, accounts, security, SmartScreen, and telemetry.
order: 90
---

# FAQ

Frequently asked questions about Waxlight. No answer here? Ask on [Discord](https://discord.gg/CrRHvg9UVw).

## General

### Is Waxlight an official Vintage Story launcher?

No. Waxlight is an independent open-source project, not affiliated with or endorsed by Anego Studios or the developers of Vintage Story. The launcher does not distribute the game or bypass its licensing: a valid Vintage Story account is required.

### Which platforms does Waxlight run on?

Windows x64 and Linux x64. Packages: installer and portable ZIP for Windows; `.deb`, `.rpm`, and portable `.tar.gz` for Linux.

### Where should I download Waxlight?

Only from the official repository's [GitHub Releases](https://github.com/AmadoMuerte/Waxlight-launcher/releases/latest) page. Every release includes `SHA256SUMS` for integrity verification.

## Installation and updates

### Windows shows a SmartScreen warning. Is it safe?

Unsigned Waxlight builds may trigger SmartScreen. Download only from the official Releases page and verify the SHA-256. See [Code signing](./policies/code-signing.md).

### Why doesn't the Windows auto-update install automatically?

By design. Because no trusted publisher is configured, the launcher rejects automatic installation of unsigned updates after checksum verification. Download the new version manually from the Releases page. See [Launcher updates](./features/updates.md).

### Where does Waxlight store its data?

Linux: `~/.config/waxlight/`; Windows: `%AppData%\waxlight\`. The folder can be relocated in **Settings → Data folder**.

## Accounts and security

### Where are my password and session stored?

Passwords and TOTP codes are never stored at all. Session keys live only in the OS store: Secret Service (GNOME Keyring/KWallet) on Linux and Credential Manager on Windows. See [Accounts & sign-in](./features/accounts.md).

### Does removing an account from the launcher end the server session?

No. Removal is local: it deletes the keys, metadata, and instance references. Vintage Story has no relied-upon session revocation endpoint.

### What does telemetry send?

By default — nothing: telemetry is off. If enabled, it sends only a pseudonymous installation ID, launcher version, OS/architecture, instance and mod counts, and predefined event names/error codes. Full list: [Privacy Policy](./policies/privacy.md).

## Game and mods

### Can I keep multiple game versions?

Yes, versions install side by side into the shared `versions/` store, and each instance gets its own version. See [Game versions](./features/game-versions.md).

### How do I share a mod build with a friend?

Export the instance as a Waxlight package: credentials and machine-specific settings are stripped automatically. Your friend imports the package and picks their own account. See [Instance packages](./features/packages.md).

### An instance broke after mod updates. What now?

Use the recovery mechanisms: safety backups and the last-known-good state. See [Backups & recovery](./features/backups.md).

## Community

### Does the project have a Discord?

Yes — the official Waxlight server: [discord.gg/CrRHvg9UVw](https://discord.gg/CrRHvg9UVw). Help, development news, feature discussions.

### How do I report a bug?

Via [GitHub Issues](https://github.com/AmadoMuerte/Waxlight-launcher/issues). Security vulnerabilities — only privately via the [security policy](./policies/security.md). Before sending, export the support log — it is redacted automatically and contains no credentials.

### How can I support the project?

Through the [support page](https://hipolink.net/amadomuerte) — Waxlight is free and developed as a labor of love.
