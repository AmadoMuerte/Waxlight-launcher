# Windows Auto-Update Architecture

## Current flow

1. When update checking is enabled, Waxlight queries GitHub Releases and, for an available update, its `SHA256SUMS` asset.
2. After the user selects **Download and Install**, Waxlight downloads the Windows installer to an update-session directory and verifies its SHA-256 value against that release asset.
3. On Windows, Waxlight then requires a valid Authenticode signature from a configured trusted publisher before it starts the installer.
4. The per-user update helper waits for Waxlight to exit, runs the NSIS installer silently, and then attempts to restart Waxlight.

The current public releases do not configure a trusted Windows publisher. Consequently, current Windows builds reject automatic installer updates after checksum verification instead of accepting unsigned code. Users must obtain such updates manually from the GitHub Releases page until SignPath Foundation signing is active.

## Trust model

`SHA256SUMS` verifies that a downloaded file matches the hash published with the GitHub release. It does not authenticate the publisher: a party able to replace both a release asset and its checksum can provide a matching malicious file.

Authenticode can provide publisher authentication and post-signing integrity, but only when Waxlight pins the expected publisher subject or certificate thumbprint. The updater fails closed when that pin is absent, the file is unsigned, the signature is invalid, or the publisher does not match.

GitHub release metadata and `update-manifest.json` are not independently authenticated by the updater. They are not a substitute for Authenticode publisher verification.

## Signing status

The release workflow does not currently perform Authenticode signing and does not use PFX secrets. SignPath Foundation integration, trusted-publisher configuration, signature verification in CI, and the required signing/repackaging order will be added only after SignPath approves the project and provides the required identifiers and artifact configuration.

The intended order is:

```text
build Waxlight executable
-> SignPath signs and CI verifies the executable
-> create portable ZIP and installer containing that signed executable
-> SignPath signs and CI verifies the installer
-> generate checksums and publish the final artifacts
```

## Installation modes

The updater detects the installation mode at runtime. Windows portable installations do not use automatic replacement. Windows installed copies receive an installer asset only after the user initiates the update.

## Diagnostics

Update progress and failures are recorded through the launcher logging system. The Windows update helper also writes its own diagnostic log under the user's local application-data directory. Do not include credentials, session material, or personal data when sharing diagnostics.
