# Vintage Story authentication

Waxlight Launcher is an unofficial launcher for Vintage Story. The upstream
authentication protocol is not publicly documented and may change. Its HTTP
implementation is isolated in `internal/auth`; application policy, native
credential storage, Wails DTOs, and game-settings injection are separate layers.

## Protocol and network boundary

Production uses fixed HTTPS endpoints:

- `POST https://auth3.vintagestory.at/v2/gamelogin`;
- `POST https://auth3.vintagestory.at/clientvalidate`.

Normal system certificate and hostname verification remains enabled. The client
has connect, TLS-handshake, response-header, and total timeouts, rejects every
redirect for credential-bearing requests, accepts only JSON responses, bounds
response bodies to 1 MiB, and performs no application retries. Test-only
construction can substitute URLs; production callers cannot configure them.
Raw response bodies and secret-bearing forms are not returned or logged.

Login sends form fields `email` and `password`. If the server requests TOTP,
Waxlight retains the password and real pre-login token in a Go-only flow for at
most five minutes. React receives an opaque random flow ID. Completion sends the
TOTP code and pre-login token from the backend. Flows are cleared on success,
cancellation, expiry, and navigation/unmount where practical. An invalid TOTP
reply retains the backend flow only so the user can retry until those limits;
the submitted code is cleared from React state after the request.

A reply is successful only when `valid == 1` and the session key, signature,
player UID, and player name are complete. Protocol failures become typed safe
statuses; raw server bodies never cross the Wails boundary.

## Stored data

SQLite `waxlight.db` contains non-secret account metadata: opaque Waxlight ID,
display/player names, email, upstream UID, validation status/timestamps, and
default selection. It contains no password, TOTP code, pre-login token, session
key, or signature.

Persistent session keys and signatures are encoded as a versioned JSON value
inside the native credential store:

```json
{"version":1,"sessionKey":"…","sessionSignature":"…"}
```

- Linux uses the freedesktop Secret Service D-Bus API, as provided by GNOME
  Keyring and compatible KWallet configurations.
- Windows uses Windows Credential Manager generic credentials.
- The service namespace is `com.waxlight.launcher`; entries use opaque Waxlight
  account IDs, not email addresses.

The adapter is `github.com/zalando/go-keyring` v0.2.8 (MIT). It was selected
because its small platform adapters call Secret Service through D-Bus and the
Windows credential API without shelling out. Its direct platform dependencies
are `github.com/godbus/dbus/v5` and `github.com/danieljoos/wincred`. Versions are
pinned in `go.mod`/`go.sum`; CI runs `govulncheck` and native integration tests.
Scanners are regression controls, not proof of security.

There is no production plaintext or in-memory fallback. Store locked, denied,
unavailable, unsupported, or corrupt conditions fail closed with a safe message.
Startup probes the store; unlocking it and restarting/retrying is required.

On POSIX, Waxlight corrects its data root and security/log directories to `0700`
and database, journals, settings, and log files to `0600`. Existing log trees are
reconciled at startup and before launch; symlinks/non-regular entries fail closed.
On Windows, sensitive paths receive a protected DACL granting access to the
current user, LocalSystem, and Administrators instead of relying on POSIX modes.

## Transaction and crash behavior

Account commit order is:

1. validate the complete authentication response;
2. record an owner-only, non-secret pending account-ID journal;
3. write the native secret;
4. atomically write metadata and default selection in a SQLite transaction;
5. clear the pending journal;
6. return an allow-listed DTO.

If metadata commit fails, a new secret is deleted; reauthentication restores the
previous secret. A startup reconciliation deletes a pending credential with no
matching metadata, covering a crash between native-store and SQLite commits. A
marker for metadata that did commit is removed while its secret is retained.
Duplicate logins reuse the account matched by upstream UID. Delete operations
are idempotent when the credential entry is already absent.

## Legacy plaintext migration

On startup Waxlight checks only the canonical
`<data-root>/account-secrets.json` path. The migrator refuses symlinks and
non-regular files, uses a 1 MiB limit and strict schema, and on POSIX requires
current-user ownership with no group/other permissions. Every account ID must
already exist in metadata.

Each value is written to native storage and read back; comparisons are constant
time where meaningful. Only after all entries verify does Waxlight overwrite
and truncate the source as a best effort, then unlink it. No plaintext backup is
created. Filesystems and storage media may retain historical blocks, snapshots,
or journal data, so this is not claimed to be cryptographic erasure.

Any parse, permission, metadata, store, or read-back failure retains the source,
records only a non-secret retry state, and blocks startup with an actionable
error. A later startup retries idempotently.

## Game launch credential lifetime

After resolving and validating the account, Waxlight writes only
`sessionkey`, `sessionsignature`, `playeruid`, and `playername` to the selected
instance's `clientsettings.json` using symlink rejection and atomic replacement.
A separate owner-only journal contains a version and timestamp, never values.
Credentials are not passed in process arguments or environment variables.

Waxlight removes all four properties after normal process exit, non-zero exit,
process-start failure, or any later launch setup failure. Startup reconciliation
clears stale properties after a launcher crash, including values written by
pre-journal versions. No-account launches also clear them. Account removal,
local logout, instance account reassignment, and retained instance deletion
clear affected settings; account removal also clears instance references.
Launches for one instance are serialized to make concurrent injection
deterministic.

Vintage Story requires these fields in plaintext while running. Malware with
the same user privileges can read them then, and a backup or filesystem snapshot
taken during play can capture them. Stop games before backup. There are
currently no export, diagnostics, or support-bundle operations; future ones are
required by CI/contributor policy to strip these properties.

## Frontend and error boundary

Wails account DTOs are allow-listed and contain metadata, status, and opaque flow
IDs only. Automated tests inspect DTO reflection and generated bindings for
prohibited fields. React does not use local/session storage or URL state for
passwords or TOTP codes, clears form values after requests/cancellation, and
uses intentional `current-password` and `one-time-code` autocomplete values.
Public application error strings omit internal causes.

## Local deletion and remote sessions

“Remove from Waxlight” deletes the native credential, metadata, references, and
instance fields. “Log out locally” is a distinct backend operation that deletes
the credential and marks retained profile metadata for reauthentication. Waxlight
does not call a verified Vintage Story revocation endpoint; neither operation is
claimed to invalidate an already issued remote session or sign out other devices.

## Verification boundary

Automated coverage includes codec/store failures, transaction compensation,
crash reconciliation, strict migration and permissions, TOTP flow expiry,
public DTO/binding leakage, network redirects/timeouts/content/size handling,
launch injection and cleanup, process args/environment, symlinks, database/log
permissions, and Linux/Windows native-store CI jobs. Windows code is also
cross-compiled locally when preparing this change. Native Windows integration
and Linux Secret Service integration require their respective CI runners; no
real Vintage Story credentials are used, so this is not an end-to-end proof
against a live account.
