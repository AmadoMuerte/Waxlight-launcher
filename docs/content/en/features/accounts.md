---
title: Accounts & sign-in
description: Multiple Vintage Story accounts with TOTP/2FA and protected session storage.
order: 10
---

# Accounts & sign-in

Multiple Vintage Story accounts with two-factor authentication support and protected session storage.

## Capabilities

- Multiple Vintage Story accounts with quick switching and a default account.
- Email/password sign-in through the official Vintage Story authentication endpoints.
- TOTP/2FA support: when the server requests a one-time code, the launcher asks for it as a separate step.
- Stored-session validation and per-account status.
- Local log out (session removed, profile metadata kept for reauthentication) and full account removal.

## How credentials are stored

Waxlight **never persists** passwords or TOTP codes. Persistent session keys live only in the operating system's credential store:

| Platform | Store |
| --- | --- |
| Linux | freedesktop Secret Service (GNOME Keyring, compatible KWallet configurations) over D-Bus |
| Windows | Windows Credential Manager (generic credentials) |

Entries use the `com.waxlight.launcher` namespace with opaque internal account IDs, not email addresses. Production builds have **no** plaintext or in-memory fallback: if the store is locked or unavailable, the launcher fails closed with an actionable message — unlock the store and retry.

> [!NOTE] A database without secrets
> The SQLite database `waxlight.db` holds only non-secret metadata: display name, email, validation status, default selection. No passwords, tokens, session keys, or signatures.

## What happens at game launch

Vintage Story requires session data in the instance's `clientsettings.json`. Waxlight:

1. writes only `sessionkey`, `sessionsignature`, `playeruid`, and `playername` via atomic file replacement with symlink rejection;
2. never passes credentials in process arguments or environment variables;
3. removes all four properties after the game exits, on launch failure, and during startup reconciliation after a possible launcher crash.

> [!WARNING] While the game is running
> Those values necessarily exist in plaintext while the game needs them — this is a game requirement. Stop games before making backups: a filesystem snapshot taken during play can capture the session.

## Removing an account

- **"Remove from Waxlight"** — deletes the credential-store entry, metadata, and all instance references.
- **"Log out locally"** — deletes the session but keeps profile metadata for fast reauthentication.

Both operations are local: Vintage Story has no relied-upon revocation endpoint, so removal in the launcher does **not** revoke an already issued server-side session or sign out other devices.

## Network and protocol

Authentication uses fixed HTTPS endpoints only (`auth3.vintagestory.at`) with normal certificate verification, timeouts, redirect rejection for credential-bearing requests, and a 1 MiB response limit. Raw server bodies and secret-bearing forms are never logged and never cross the backend boundary. Technical details: [authentication.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/authentication.md) and the [Security](../policies/security.md) page.
