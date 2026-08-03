# Security policy

## Reporting a vulnerability

Please do not disclose security vulnerabilities, authentication data, or account
session material in a public GitHub issue.

Use GitHub's private vulnerability reporting feature for this repository. If it
is unavailable, contact the repository owner through their GitHub profile and
ask for a private reporting channel. Include the affected Waxlight version,
operating system, reproduction steps, and impact. Remove passwords, session
keys, signatures, tokens, email addresses, and other personal data from logs.

We will acknowledge a report as soon as practical, reproduce and assess it,
prepare a fix and regression test where feasible, and coordinate disclosure
after users have a reasonable upgrade path. This is a small independent project,
so no fixed response or remediation SLA is promised.

Proofs of concept should use synthetic sentinel values. Before sharing logs,
archives, screenshots, databases, or `clientsettings.json`, remove passwords,
TOTP codes, pre-login tokens, session keys/signatures, full email addresses, and
other personal data. Do not send real credentials even through the private
reporting channel; arrange a narrower secure exchange only if it is essential.

## Scope

Security-sensitive areas include:

- Vintage Story authentication and TOTP flows;
- local session credential storage;
- archive extraction and downloaded file verification;
- game and mod installation paths;
- process launch arguments;
- update and release artifacts.

Waxlight stores persistent session credentials in Secret Service-compatible
storage on Linux and Windows Credential Manager on Windows. Production does not
fall back to plaintext files. Passwords and TOTP codes are never persisted.
Legacy `account-secrets.json` files are read only by a constrained migration and
are removed only after every imported value is read back and verified.

The security scope also includes credentials temporarily injected into a game
instance's `clientsettings.json`. Those values are removed after exit/failure and
on startup reconciliation, but they necessarily exist in plaintext while the
game needs them. Support-bundle, diagnostics, and instance-export features do
not currently exist; any future implementation must remove the four
authentication properties before producing an artifact.

## Threat-model boundary

The implementation is intended to resist other unprivileged local OS users,
accidental inclusion in normal project data, malformed server responses,
unsafe migration files, and frontend attempts to retrieve persistent secrets.
It cannot protect secrets from malware running with the same user privileges, a
compromised kernel, an attached debugger, a hostile credential-store
implementation, or a malicious build artifact. No desktop application can
protect credentials on a fully compromised host.

Deleting an account from Waxlight removes local credentials and instance
references. Vintage Story has no relied-upon revocation endpoint in this
implementation, so local deletion must not be interpreted as remote session
revocation.

## Supported versions

Waxlight is currently an early preview. Security fixes are provided for the most
recent published release only.
