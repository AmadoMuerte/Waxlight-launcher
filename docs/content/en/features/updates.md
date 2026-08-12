---
title: Launcher updates
description: Stable and prerelease channels, SHA-256 verification, trusted-publisher control.
order: 80
---

# Launcher updates

Updates you control: stable and prerelease channels, checksum and signature verification.

## How it works

1. When update checking is enabled, Waxlight queries GitHub Releases and, for an available update, its `SHA256SUMS` asset.
2. After you choose "Download and Install", the launcher downloads the installer and verifies its SHA-256 against the release data.
3. On Windows, a valid Authenticode signature from a configured trusted publisher is additionally required before the installer is started.

Automatic update checking is enabled by default and runs when the launcher starts; it can be disabled in **Settings → Updates**. You can choose the channel: stable or prerelease.

## Trust model

> [!WARNING] SHA-256 ≠ publisher signature
> `SHA256SUMS` confirms that a file matches the published hash, but does not authenticate the publisher: anyone able to replace both the file and its checksum can serve a matching malicious file.

Authenticode provides publisher authentication and post-signing integrity — but only when Waxlight pins the expected publisher (certificate subject or thumbprint). If the pin is absent, the file is unsigned, the signature is invalid, or the publisher does not match, the update is **rejected**.

## Current signing status

Windows releases are currently unsigned. Because no trusted Windows publisher is configured, Windows builds **refuse automatic update installation** after checksum verification. Update manually from the [GitHub Releases](https://github.com/AmadoMuerte/Waxlight-launcher/releases/latest) page.

Details on the [Code signing](../policies/code-signing.md) page.

## Installation modes

The updater detects the installation mode at runtime. Windows portable copies are not replaced automatically; installed copies receive an installer only after the user initiates the update. The update helper waits for Waxlight to exit, runs the installer silently, and attempts to restart the launcher.

## SmartScreen and MOTW

The updater does **not** use `Unblock-File`: removing the Mark of the Web bypasses SmartScreen protection. Manual downloads keep Windows' normal MOTW and SmartScreen behavior.

Technical document: [windows-updater.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/windows-updater.md).
