---
title: Code signing
description: Current Windows release signing status and artifact verification.
order: 30
---

# Code signing policy

Windows releases are currently unsigned.

Official Waxlight release artifacts are built by GitHub Actions from tagged commits in the public repository and published through [GitHub Releases](https://github.com/AmadoMuerte/Waxlight-launcher/releases/latest). Each release provides `SHA256SUMS` for integrity verification.

`SHA256SUMS` verifies that a download matches the release checksum, but it does not provide publisher authentication. Unsigned Windows builds may trigger Microsoft Defender SmartScreen.

Waxlight may introduce Windows code signing in the future. If that happens, this policy and the release pipeline will be updated to describe the actual signing mechanism.

Download Waxlight only from the official GitHub Releases page. Report security issues related to release provenance, signing, or sensitive data using the [security policy](./security.md) process.
