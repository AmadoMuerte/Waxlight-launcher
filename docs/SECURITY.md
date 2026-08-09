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
game needs them. Waxlight can export a redacted support log and import/export
instances as Waxlight packages. Support logs redact sensitive log values and do
not include credentials, account data, or private paths. Instance packages
sanitize `clientsettings.json` and exclude session keys, session signatures,
player IDs/names, MP tokens, passwords, credential-store secrets, authentication
tokens, and other account or machine-specific settings. Do not add any of those
values to support exports or ordinary instance exports.

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

## Windows release signing policy

The project-wide [Code signing policy](CODE_SIGNING_POLICY.md) defines the source, roles, approval requirements, privacy links, and artifact rules used for Windows release signing.

Waxlight is transitioning Windows release signing to the SignPath Foundation open-source signing program. Releases published before that integration is active may be unsigned. A Git tag marked “Verified” by GitHub is not the same thing as an Authenticode signature on a Windows executable. Always verify the specific downloaded artifact.

For releases signed through the Foundation program: **Free code signing provided by SignPath.io, certificate by SignPath Foundation.**

### What SHA-256 verifies vs what Authenticode verifies

SHA-256 checksums in `SHA256SUMS` confirm that the downloaded file is byte-identical to the artifact for which the checksum was generated. SHA-256 does **not** establish publisher identity.

Authenticode adds publisher authentication and post-signing integrity. A signed Waxlight release should report a valid Windows Authenticode signature and a certificate chain trusted by Windows. Legacy unsigned releases will report `NotSigned`.

### Trusted publisher configuration

The launcher updater requires a configured trusted-publisher list before it will install a Windows update automatically. Until the production SignPath publisher identity is configured in release builds, it rejects automatic Windows installer updates after checksum verification. Checksum verification must not be described as equivalent to publisher authentication.

### Why Unblock-File is not used

`Unblock-File` (PowerShell) removes the Mark of the Web (MOTW) alternate data
stream from downloaded files. MOTW is set by browsers and Windows for files
downloaded from the internet, and SmartScreen uses it to decide whether to warn
the user before running the file.

Removing MOTW bypasses SmartScreen protection. The updater does **not** use
`Unblock-File`. It only starts Windows installers after trusted-publisher
Authenticode verification succeeds; while no publisher is configured, automatic
Windows installation is refused. Manual downloads retain Windows' normal MOTW
and SmartScreen behavior.

## Supported versions

Waxlight is currently an early preview. Security fixes are provided for the most
recent published release only.
