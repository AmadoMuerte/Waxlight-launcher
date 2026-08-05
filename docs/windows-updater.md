# Windows Auto-Update Architecture

## Current Flow

```
Frontend (App.tsx)
  → updatesApi.check(channel)
  → Wails binding
  → LauncherUpdateController.CheckUpdates
  → LauncherUpdateService.Check
  → Source.Check (GitHub API + SHA256SUMS)
  → returns LauncherUpdate DTO

User clicks "Download and Install"
  → updatesApi.install(channel)
  → LauncherUpdateController.InstallUpdate
  → LauncherUpdateService.Install
    → Source.Check (re-fetch)
    → validate asset name + checksum length
    → Downloader.Download (HTTP → .partial → atomic rename → SHA-256 verify)
    → SignatureVerifier.Verify (Authenticode + publisher check)
    → Installer.Apply (Windows: exec.Command with /S /CURRENT_PID)
    → publish "restarting" phase
  → controller: sleep 250ms → wruntime.Quit
```

## Participating Files

| Layer | File | Responsibility |
|-------|------|----------------|
| Domain | `internal/domain/update.go` | `LauncherUpdate`, `LauncherUpdateProgress` structs |
| Domain | `internal/domain/errors.go` | `AppError`, update error constants |
| Application | `internal/application/ports.go` | `Downloader`, `LauncherUpdateSource`, `LauncherUpdateInstaller` interfaces |
| Application | `internal/application/updater.go` | `LauncherUpdateService` — orchestrates check/download/install |
| Infrastructure | `internal/infrastructure/updater/source.go` | GitHub Releases API, asset selection, SHA256SUMS parsing |
| Infrastructure | `internal/infrastructure/updater/installer.go` | Cross-platform `Installer` type |
| Infrastructure | `internal/infrastructure/updater/installer_windows.go` | Windows: exec.Command with NSIS `/S /CURRENT_PID` args |
| Infrastructure | `internal/infrastructure/updater/signature.go` | `SignatureVerifier` interface |
| Infrastructure | `internal/infrastructure/updater/signature_windows.go` | Windows Authenticode verification via PowerShell |
| Infrastructure | `internal/infrastructure/updater/installer_linux.go` | Linux: atomic binary replacement or xdg-open |
| Infrastructure | `internal/infrastructure/downloader/http.go` | HTTP download with resume, checksum, atomic write |
| Infrastructure | `internal/infrastructure/downloader/manager.go` | Concurrency-limited download wrapper |
| Presentation | `internal/presentation/update_controller.go` | Wails-bound controller, progress events |
| Bootstrap | `internal/bootstrap/container.go` | Dependency wiring |
| Frontend | `frontend/src/app/App.tsx` | Update banner, progress bar, user actions |
| Frontend | `frontend/src/shared/api/types.ts` | TypeScript DTOs |
| Build | `scripts/build-windows.ps1` | Windows NSIS build, asset packaging |
| CI | `.github/workflows/release.yml` | Release pipeline, Authenticode signing, SHA256SUMS generation |

## Trust Model

### What SHA-256 Verifies

SHA-256 checksum in `SHA256SUMS` confirms the downloaded file is byte-identical to the file the release publisher had. It protects against:

- Corrupted downloads
- CDN tampering
- Network bit-flip errors

### What SHA-256 Does NOT Verify

SHA-256 does NOT verify:

- **Who created the file.** If the GitHub account is compromised, the attacker publishes valid SHA-256 hashes for malicious binaries.
- **That the file is a legitimate installer.** Any executable can be hashed.
- **That the file is safe.** SHA-256 is a integrity check, not a security assessment.

### What Authenticode Verifies

Authenticode (digital signature) verifies:

- **Publisher identity.** The signing certificate identifies who built the file. The certificate subject and thumbprint are checked against a configured list of trusted publishers.
- **Integrity since signing.** Any modification after signing invalidates the signature.
- **Certificate chain.** The certificate chains to a trusted Certificate Authority.
- **Timestamp.** The signature includes a timestamp from a trusted timestamp authority, proving the signature was valid at signing time even if the certificate later expires.

SHA-256 + Authenticode together provide integrity + authenticity. Neither alone is sufficient.

## Authenticode Signing Workflow

### How Signing Works in CI

The release pipeline in `.github/workflows/release.yml` performs Authenticode signing:

1. Builds standalone `.exe`, portable `.zip`, and NSIS installer `.exe`
2. Restores the code-signing certificate from `CODESIGN_CERTIFICATE_BASE64`
3. Decodes and imports the certificate into the CI runner's certificate store
4. Signs the standalone `.exe` and installer `.exe` using `signtool.exe sign`
5. Generates `SHA256SUMS` with `sha256sum`
6. Publishes all artifacts to GitHub Releases
7. Cleans up the imported certificate

Signing is performed on the following artifacts:

- `Waxlight-Launcher-vX.Y.Z-windows-amd64.exe` (standalone)
- `Waxlight-Launcher-vX.Y.Z-windows-amd64-installer.exe` (NSIS installer)

The portable `.zip` archive does not require signing because it contains the raw executable, which is signed individually.

### How Signature Verification Works

The updater verifies Authenticode signatures before launching the installer:

1. After SHA-256 checksum verification passes, the updater calls `SignatureVerifier.Verify()`
2. On Windows, this invokes PowerShell `Get-AuthenticodeSignature -LiteralPath <path>`
3. The result is parsed into a `powershellSignatureResult` struct containing:
   - `Status`: `Valid`, `NotSigned`, `HashMismatch`, `NotTrusted`, or other
   - `SignerCertificate.Subject`: The certificate subject (e.g., `CN=Developer Name`)
   - `SignerCertificate.Thumbprint`: The certificate thumbprint
4. If the status is `Valid`, the subject or thumbprint is matched against the configured trusted publishers list
5. If the status is `NotSigned` or `HashMismatch`, the download is deleted and the update is aborted
6. If the status is `NotTrusted`, the download is deleted and the update is aborted
7. If no trusted publishers are configured, `ErrNoTrustedPublisher` is returned

Verification failures return these errors:

| Error | Meaning |
|-------|---------|
| `ErrSignatureMissing` | File is not signed |
| `ErrSignatureInvalid` | Signature is invalid or tampered |
| `ErrSignatureNotTrusted` | Signature is not trusted by Windows (certificate chain issue) |
| `ErrNoTrustedPublisher` | No trusted publishers configured in the updater |
| `ErrPublisherMismatch` | Certificate subject/thumbprint does not match any trusted publisher |

## Trusted Publisher Configuration

The updater accepts a list of trusted publishers as certificate subjects or thumbprints. A publisher is matched if either the `Subject` or `Thumbprint` field matches exactly.

### Recommended Configuration

Trusted publishers are configured at startup in `internal/bootstrap/container.go` and passed to `NewSignatureVerifier()`. Example publishers:

- `CN=Your Organization Name, O=Your Organization, C=US` (full subject)
- `A1B2C3D4E5F6...` (thumbprint — 40-character hex string)

### Publisher Rotation

To rotate signing certificates:

1. Generate a new certificate and add it to `CODESIGN_CERTIFICATE_BASE64`
2. Before publishing a release with the new certificate, update the trusted publishers list in the updater configuration
3. Publish the release — users running older updater versions will not trust the new signature
4. After a reasonable overlap period, remove the old publisher from the trusted list

## Portable Mode Handling

The updater detects portable installations and blocks automatic updates:

1. `LauncherUpdateService.Install()` checks `update.InstallationMode`
2. If the mode is `"portable"`, the updater returns `ErrUpdateUnsupported` with a message directing the user to download the new portable package manually
3. The session directory is cleaned up and no download occurs
4. The UI displays the message: "Automatic replacement is unavailable for portable installations. Download the new portable package and replace the current version manually."

Portable mode is determined at build time — portable builds do not produce an NSIS installer and use a different update asset selection path in `source.go`.

## Update Stages

The updater progresses through these stages:

| Stage | Constant | Description |
|-------|----------|-------------|
| `checking` | `UpdateStageChecking` | Fetching latest release info from GitHub |
| `downloading` | `UpdateStageDownloading` | Downloading the installer to a session directory |
| `hash_verification` | `UpdateStageHashVerification` | Verifying SHA-256 checksum (done by downloader) |
| `signature_verification` | `UpdateStageSignatureCheck` | Verifying Authenticode signature and publisher |
| `starting_installer` | `UpdateStageStartingInstaller` | Launching the NSIS installer |
| `closing_application` | `UpdateStageClosingApplication` | Waiting for installer to signal launcher to exit |
| `restarting` | `UpdateStageRestarting` | Launcher is exiting to allow installer to complete |

## Error Codes

All update error codes are defined in `internal/domain/errors.go`:

| Code | Constant | Meaning | Retryable |
|------|----------|---------|-----------|
| `UPDATE_UNAVAILABLE` | `ErrUpdateUnavailable` | No update available or API error | Yes |
| `UPDATE_ALREADY_IN_PROGRESS` | `ErrUpdateInProgress` | Another update is running | No |
| `UPDATE_FAILED` | `ErrUpdateFailed` | General update failure (invalid filename, bad checksum) | No |
| `UPDATE_DOWNLOAD_FAILED` | `ErrUpdateDownloadFailed` | Download failed (network, server error) | Yes |
| `UPDATE_CHECKSUM_MISMATCH` | `ErrUpdateChecksumMismatch` | SHA-256 mismatch after download | No |
| `UPDATE_SIGNATURE_MISSING` | `ErrUpdateSignatureMissing` | Downloaded file is not signed | No |
| `UPDATE_SIGNATURE_INVALID` | `ErrUpdateSignatureInvalid` | Signature is invalid or tampered | No |
| `UPDATE_PUBLISHER_MISMATCH` | `ErrUpdatePublisherMismatch` | Signed by unknown publisher | No |
| `UPDATE_INSTALLER_BLOCKED` | `ErrUpdateInstallerBlocked` | Installer was blocked (policy, antivirus) | No |
| `UPDATE_INSTALLER_START_FAILED` | `ErrUpdateInstallerStartFail` | Failed to start installer process | No |
| `UPDATE_INSTALLER_EXITED_EARLY` | `ErrUpdateInstallerExited` | Installer exited before completing | No |
| `UPDATE_UNSUPPORTED_INSTALLATION` | `ErrUpdateUnsupported` | Portable mode — manual update required | No |
| `UPDATE_RESTART_FAILED` | `ErrUpdateRestartFailed` | Failed to trigger application restart | No |

### Windows Installer Start Errors

When `exec.Command(installerPath, "/S", "/CURRENT_PID=...").Start()` fails, the error is classified:

| Scenario | Error Type |
|----------|------------|
| File not found | `ErrInstallerNotFound` |
| Access denied | `ErrInstallerAccessDenied` |
| Not a valid Win32 application | `ErrInstallerInvalid` |
| Administrator privileges required | `ErrInstallerElevationRequired` |
| Antivirus quarantined | File disappears between download and exec (unclassified) |
| User cancelled UAC elevation | `ErrInstallerElevationRequired` |

## GitHub Secrets for Signing

The following secrets must be configured in the GitHub repository:

| Secret | Description |
|--------|-------------|
| `CODESIGN_CERTIFICATE_BASE64` | Base64-encoded `.pfx` certificate file containing the private key |
| `CODESIGN_PASSWORD` | Password for the `.pfx` certificate |

### Certificate Requirements

- Must be a valid Authenticode code-signing certificate
- Should be issued by a trusted Certificate Authority (not self-signed)
- For EV (Extended Validation) certificates, the certificate must be stored on a hardware token — the CI workflow must use an HSM-backed signing solution
- The certificate should have a validity period covering the expected release window

### Setting Up Secrets

1. Obtain a code-signing certificate from a Certificate Authority
2. Export the certificate and private key as a `.pfx` file
3. Base64-encode the `.pfx` file: `[Convert]::ToBase64String([IO.File]::ReadAllBytes("certificate.pfx"))`
4. In GitHub repository settings, go to **Secrets and variables → Actions**
5. Add `CODESIGN_CERTIFICATE_BASE64` with the base64-encoded content
6. Add `CODESIGN_PASSWORD` with the certificate password

## Certificate Rotation

### When to Rotate

- The certificate is approaching its expiration date
- The private key may have been compromised
- You want to change the signing identity

### Rotation Procedure

1. **Before rotation:** Add the new certificate's publisher identity to the trusted publishers list in a new release of the updater. This ensures users with older updater versions can verify the new signature.
2. **Obtain the new certificate** from your Certificate Authority
3. **Update GitHub Secrets** with the new certificate:
   - Replace `CODESIGN_CERTIFICATE_BASE64` with the new `.pfx` (base64-encoded)
   - Replace `CODESIGN_PASSWORD` with the new password
4. **Remove the old certificate** from the signing pipeline (only after the trusted publishers list update has been deployed)
5. **Publish a test release** to verify the new signature works
6. **Remove the old publisher** from the trusted publishers list in a subsequent release

### Rollback

If the new certificate causes verification failures:

1. Revert the GitHub Secrets to the previous certificate values
2. Publish a hotfix release signed with the old certificate
3. Investigate the verification failure before attempting rotation again

## How Installation Works

### Windows

1. Download installer to `{dataRoot}/updates/{sessionID}/{assetName}`
2. Verify SHA-256 checksum (handled by downloader)
3. Verify Authenticode signature and publisher (via PowerShell `Get-AuthenticodeSignature`)
4. Start `exec.Command(installerPath, "/S", "/CURRENT_PID=<pid>")` with hidden window
5. Sleep 250ms
6. Call `wruntime.Quit(ctx)` to exit the launcher
7. NSIS installer replaces files after launcher exits

### NSIS Installer Arguments

The updater passes two arguments to the NSIS installer:

- `/S` — Silent mode (no UI)
- `/CURRENT_PID=<pid>` — PID of the running launcher, allowing the installer to wait for the launcher to exit before replacing files

## How to Diagnose Update Errors

### Current Logging

- `updater.go`: Logs "checking for update", "downloading update", checksum mismatch, installer errors
- `source.go`: Logs API errors, checksum parse failures
- `downloader/http.go`: Logs download errors, resume attempts
- `signature_windows.go`: Logs PowerShell execution errors, signature parse failures

### Structured Update Stages

The updater publishes structured progress events with named stages:

```
checking → downloading → hash_verification → signature_verification → installing → restarting
```

Each stage includes a phase name and optional progress percentage. Frontend can use the phase name to display appropriate UI state.

### Missing Diagnostics

- No Windows error code classification in structured form
- No update session tracking across restarts
- No installer PID logging after start
- No publisher information logging on success
- No retry-after headers from GitHub API
