# Vintage Story authentication

Waxlight Launcher is an unofficial launcher for Vintage Story. The
authentication endpoints described here are not publicly documented by Vintage
Story and may change without notice. They are isolated in `internal/auth` so a
protocol change does not leak into Wails controllers, React, account persistence,
or the process launcher.

## Protocol

Login sends `POST https://auth3.vintagestory.at/v2/gamelogin` with
`application/x-www-form-urlencoded` fields `email` and `password`. When the
server returns `requiretotpcode`, Waxlight keeps email, password, and the real
`prelogintoken` only in an in-memory five-minute flow. React receives only a
random UUID flow ID. Completion adds `totpcode` and `prelogintoken` to the same
form request.

A login is successful only when `valid == 1` and `sessionkey`,
`sessionsignature`, `uid`, and `playername` are all present. HTTP 200 alone is
not considered success. Server reasons are converted to typed internal errors
and discriminated frontend statuses; raw replies are never exposed.

Session validation uses `POST https://auth3.vintagestory.at/clientvalidate`
with form fields `uid` and `sessionkey`. On 2026-08-02 the live endpoint was
probed with deliberately invalid placeholder credentials: POST form data was
parsed by the server (`missingaccount`), while both GET query and GET form-body
variants returned `missinguidorsessionkey`. This is why Waxlight intentionally
differs from Grunt Launcher's ambiguous `GET(...).form(...)` call. An
`httptest.Server` contract test locks the method, content type, and fields.

## Architecture

- `internal/auth` owns URLs, HTTP, response parsing, and typed protocol errors.
- `application.AccountService` owns login flows, account selection, validation,
  deduplication by server UID, and safe results.
- `SQLiteStore` owns non-secret account metadata and the selected account.
- `credentials.FileStore` owns session key/signature persistence.
- `filesystem.ClientSettingsService` patches only authentication properties in
  the instance's `clientsettings.json`.
- Wails `AccountController` exposes only high-level account use cases and DTOs.

The Wails account API consists of `Login`, `CompleteTOTP`, `CancelLogin`,
`ListAccounts`, `SetDefaultAccount`, `RemoveAccount`, `ValidateAccount`, and
`ReauthenticateAccount`. No method returns session credentials or a credential
store path.

## Secret storage and security notes

Passwords and TOTP codes are never persisted. They are cleared from React state
after each request. A TOTP flow's password and pre-login token exist only in the
Go process, expire after five minutes, and are cleared on success or cancellation.
None of these values are logged.

Account metadata is stored in `waxlight.db`. `sessionkey` and
`sessionsignature` are stored separately in `account-secrets.json`, written via
temporary file, `fsync`, and atomic replacement. The file is mode `0600` on
POSIX systems and is excluded from exports and diagnostics. This cross-platform
file store is a temporary fallback: system Secret Service/libsecret and Windows
Credential Manager adapters are still required before treating local at-rest
protection as complete. Corrupt secret files are rejected and backed up instead
of silently discarded.

Removing an account deletes its local secrets and metadata and clears database
foreign-key references from instances. It does not revoke the server session or
sign the user out on other devices.

## Launch pipeline

Waxlight resolves the requested account, then the instance default, then the
globally selected account. If an account is selected, the launcher loads its
secret, validates it, and stops on expiration or any validation error. A network
failure is retryable and does not change the account to expired.

Only after successful validation does Waxlight atomically patch
`<instance-data-path>/clientsettings.json` with `sessionkey`,
`sessionsignature`, `playeruid`, and `playername`. Existing JSON properties are
preserved. Invalid JSON or an invalid `stringsettings` value blocks launch. A
launch without an account clears those four fields to prevent accidentally
reusing a previous player's session. The game is then started with the
instance's isolated `--dataPath`, and its play session records the resolved
account ID.

## Verification boundary

Unit and integration tests cover protocol parsing, TOTP continuation, metadata
and secret persistence, permissions, validation behavior, settings patching,
and process-launch blocking. No real Vintage Story account was used, so this
implementation must not be described as verified end-to-end against a real
account.
