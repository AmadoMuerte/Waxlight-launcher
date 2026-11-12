# Privacy Policy

**Last updated: November 12, 2026**

Waxlight Launcher ("Waxlight") is an independent, open-source launcher for Vintage Story. This policy describes what Waxlight stores locally and which third-party services Waxlight contacts.

## 1. Local data

Waxlight stores launcher settings, installed game versions, instances, mods, downloads, logs, playtime records, and other launcher state on the user's device. The default data directory is the operating system configuration directory and can be relocated from Waxlight settings.

Persistent Vintage Story session credentials are stored through the operating system's native credential store. Passwords and TOTP codes are not persisted by Waxlight. Authentication values temporarily required by Vintage Story may be injected into an instance configuration while the game is running and are cleaned up by the launcher afterwards.

Waxlight does not collect or transmit usage telemetry. The launcher does not send heartbeats, analytics events, or error reports to a Waxlight-operated service.

## 2. Third-party services

Waxlight contacts the following services. Information sent directly to those services is governed by their respective privacy policies.

- **Vintage Story / Anego Studios** for authentication, account validation, game-version discovery/downloads, and ModDB functionality when those launcher features are used. Vintage Story Privacy Policy: https://www.vintagestory.at/privacy/
- **GitHub** for Waxlight release information, downloads, and update discovery. Automatic update checking is enabled by default and checks GitHub when the launcher starts; it can be disabled in **Settings → Updates**. GitHub Privacy Statement: https://docs.github.com/site-policy/privacy-policies/github-privacy-statement

## 3. Security and transparency

Waxlight is open source. Launcher networking, release verification, and update checks are available for public review in this repository.

Security vulnerabilities or suspected exposure of sensitive information should be reported using the private process described in [SECURITY.md](SECURITY.md). Do not post credentials, tokens, or other sensitive information in a public issue.

## 4. Privacy requests and contact

For privacy questions or requests, contact the repository owner through the GitHub profile and request a private communication channel:

https://github.com/AmadoMuerte

## 5. Changes to this policy

This policy may be updated when Waxlight's data practices, infrastructure, or legal obligations change. Material changes will be published in this repository.
