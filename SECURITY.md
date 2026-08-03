# Security policy

## Reporting a vulnerability

Please do not disclose security vulnerabilities, authentication data, or account
session material in a public GitHub issue.

Use GitHub's private vulnerability reporting feature for this repository. If it
is unavailable, contact the repository owner through their GitHub profile and
ask for a private reporting channel. Include the affected Waxlight version,
operating system, reproduction steps, and impact. Remove passwords, session
keys, signatures, tokens, email addresses, and other personal data from logs.

We will acknowledge a report as soon as practical, investigate it, and coordinate
disclosure after a fix is available.

## Scope

Security-sensitive areas include:

- Vintage Story authentication and TOTP flows;
- local session credential storage;
- archive extraction and downloaded file verification;
- game and mod installation paths;
- process launch arguments;
- update and release artifacts.

Waxlight currently stores session credentials in an owner-readable local file as
a fallback. Passwords and TOTP codes are never persisted. Native Secret Service
and Windows Credential Manager integrations are planned.

## Supported versions

Waxlight is currently an early preview. Security fixes are provided for the most
recent published release only.
