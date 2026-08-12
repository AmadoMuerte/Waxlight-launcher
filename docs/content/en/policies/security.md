---
title: Security
description: Threat model, credential storage, and how to report a vulnerability privately.
order: 20
---

# Security policy

How Waxlight protects credentials, and how to report a vulnerability privately.

*Canonical source: [docs/SECURITY.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/SECURITY.md).*

## Reporting a vulnerability

> [!WARNING] Do not disclose vulnerabilities in public issues
> Please do not disclose security vulnerabilities, authentication data, or account session material in a public GitHub issue.

Use GitHub's **private vulnerability reporting** feature for this repository. If it is unavailable, contact the owner through their [GitHub profile](https://github.com/AmadoMuerte) and ask for a private reporting channel. Include the affected Waxlight version, operating system, reproduction steps, and impact. Remove passwords, session keys, signatures, tokens, email addresses, and other personal data from logs.

We will acknowledge a report as soon as practical, reproduce and assess it, prepare a fix and regression test where feasible, and coordinate disclosure after users have a reasonable upgrade path. This is a small independent project, so no fixed response or remediation SLA is promised.

Proofs of concept should use synthetic sentinel values. Do not send real credentials even through the private channel; a narrower secure exchange is arranged only if essential.

## Scope

Security-sensitive areas include:

- Vintage Story authentication and TOTP flows;
- local session credential storage;
- archive extraction and downloaded file verification;
- game and mod installation paths;
- process launch arguments;
- update and release artifacts.

Persistent session credentials are stored in Secret Service-compatible storage on Linux and Windows Credential Manager on Windows. Production does not fall back to plaintext files. Passwords and TOTP codes are never persisted. Legacy `account-secrets.json` files are read only by a constrained migration and removed only after every imported value is read back and verified.

## Threat-model boundary

The implementation is intended to resist: other unprivileged local OS users, accidental inclusion of secrets in normal project data, malformed server responses, unsafe migration files, and frontend attempts to retrieve persistent secrets.

It **cannot** protect secrets from: malware running with the same user privileges, a compromised kernel, an attached debugger, a hostile credential-store implementation, or a malicious build artifact. No desktop application can protect credentials on a fully compromised host.

Deleting an account from Waxlight removes local credentials and instance references. Vintage Story has no relied-upon revocation endpoint in this implementation, so local deletion must not be interpreted as remote session revocation.

## Exports and logs

- Waxlight can export a **redacted support log**: sensitive values are stripped; credentials, account data, and private paths are never included.
- **Instance packages** sanitize `clientsettings.json` and exclude session keys, session signatures, player IDs/names, MP tokens, passwords, credential-store secrets, and authentication tokens.
- None of those values may be added to support exports or ordinary instance exports — this is a hard project rule.

## Supported versions

Waxlight is currently an early preview. Security fixes are provided for the most recent published release only.

## Windows release signing

Windows release artifacts are currently unsigned. Release verification guidance is defined by the [code signing policy](./code-signing.md). A "Verified" Git tag on GitHub is not the same as an Authenticode signature on an executable.
