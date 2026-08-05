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

## Windows release signing policy

All Windows release artifacts (standalone executable and NSIS installer) are
signed with Authenticode digital signatures. The signing certificate is stored
as a GitHub Actions secret (`CODESIGN_CERTIFICATE_BASE64`) and imported into
the CI runner's certificate store for each release build.

### What SHA-256 verifies vs what Authenticode verifies

SHA-256 checksums in `SHA256SUMS` confirm that the downloaded file is
byte-identical to the file the release publisher had when the checksum was
generated. This protects against corrupted downloads, CDN tampering, and
network bit-flip errors. SHA-256 does **not** verify who created the file. If
the GitHub account is compromised, the attacker can publish valid SHA-256 hashes
for malicious binaries.

Authenticode verifies that the file was signed by a trusted publisher whose
certificate chains to a trusted Certificate Authority. It confirms publisher
identity and that the file has not been modified since signing. Together,
SHA-256 + Authenticode provide integrity + authenticity. Neither alone is
sufficient.

### Trusted publisher configuration

The launcher updater maintains a configured list of trusted publishers —
certificate subjects or thumbprints that are accepted during signature
verification. Only files signed by one of these publishers are installed. The
updater rejects unsigned files, files signed by unknown publishers, and files
whose signature is invalid or untrusted by Windows.

Trusted publishers are updated before new signing certificates are introduced,
so older launcher versions can verify releases signed with the new certificate.

### Why Unblock-File is not used

`Unblock-File` (PowerShell) removes the Mark of the Web (MOTW) alternate data
stream from downloaded files. MOTW is set by browsers and Windows for files
downloaded from the internet, and SmartScreen uses it to decide whether to warn
the user before running the file.

Removing MOTW bypasses SmartScreen protection. The updater does **not** use
`Unblock-File`. Instead, it relies on Authenticode signatures: SmartScreen
recognizes signed executables from trusted publishers and does not display
unnecessary warnings. This approach maintains the operating system's security
boundaries rather than working around them.

## Supported versions

Waxlight is currently an early preview. Security fixes are provided for the most
recent published release only.
