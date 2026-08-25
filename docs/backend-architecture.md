# Waxlight Backend Architecture

This document describes the final feature-oriented architecture of the Waxlight
backend. It is the reference for every agent working on the repository: read
the package map, the dependency rules, and the extension checklist before
changing backend code.

## Package Map

All backend code lives under `internal/` plus the small executable entrypoint
`cmd/waxlight`.

### Composition root

| Package | Responsibility |
|---|---|
| `internal/app` | The complete, explicit composition root. `wire.go` constructs every dependency exactly once and assembles the `Container`; `Startup`/`Shutdown` own the framework-facing lifecycle. `adapters.go` holds the store-to-feature mappings that belong to no feature. |
| `internal/apptest` | Test-only lifecycle helper for feature tests that must not import the composition root. |

### Features

Features own domain models, ports (interfaces), use cases, and their tests.
They never import Wails, SQLite, platform adapters, or each other's internals —
only through narrow ports and structural views.

| Package | Responsibility |
|---|---|
| `internal/accounts` | Account models, login/TOTP flows, native credential-store ports, account persistence. |
| `internal/instances` | Instance model, CRUD, creation, cloning, package import/export (`PackageService`), instance-directory removal policy (`SafeRemoveAll`), and the `.waxlight` package models. |
| `internal/versions` | Installed/available game versions, local and catalog installation, removal, executable repair. |
| `internal/launching` | Launch validation, start/stop/tracking, credential injection into `clientsettings.json`, game events, launch recovery port. |
| `internal/sessions` | Play-session persistence, interrupted-session recovery, statistics inputs. |
| `internal/mods` | Installed/downloaded mod orchestration, ModDB catalog browsing, ModDB task manager, dependency resolution. |
| `internal/snapshots` | Manual and automatic safety snapshots, restore coordination, retention, exact mod restoration. |
| `internal/recovery` | Last Known Good markers, failed-launch analysis, recovery suggestions. |
| `internal/servers` | Favorite-server persistence and public-catalog queries. |
| `internal/settings` | Launcher settings, data-root relocation coordinator. |
| `internal/operations` | Persistent operations (status, progress, workers, cancellation, futures). |
| `internal/updates` | Launcher update checks, verification, installation, skip policy. |
| `internal/telemetry` | Privacy-consented telemetry with allowlists, worker-group delivery. |
| `internal/statistics` | Aggregated statistics and per-instance playtime over the sessions reader. |
| `internal/gamelog` | Game-output tailing, crash classification, launcher-side crash reports. |
| `internal/downloads` | Transport-neutral download contract. |
| `internal/events` | Framework-neutral publisher abstraction. |
| `internal/mutations` | Global write gate (`Gate`) and per-instance busy slot (`Slot`). |
| `internal/errs` | The shared UI-facing error contract: `AppError`, `NewError`, `ErrNotFound`, and every error code. Code strings are a frontend contract (i18n keys) and must never change. |
| `internal/version` | Build version metadata. |
| `internal/language` | Supported language list for settings validation. |
| `internal/publishers` | Trusted Windows publisher identifiers for update signature checks. |

### Transport

| Package | Responsibility |
|---|---|
| `internal/transport/wails` | All Wails controllers and DTOs, feature-oriented files. Limited to parameter/DTO conversion, feature invocation, dialogs, events, browser/directory opening, and quit. Controllers depend on features, never on platform/SQLite. |

### Platform adapters

| Package | Responsibility |
|---|---|
| `internal/platform/sqlite` | The single shared `*sql.DB`, versioned transactional migrations, and repository implementations for every feature. |
| `internal/platform/snapshots` | Snapshot storage on disk: staging, manifests, listing, removal. |
| `internal/platform/process` | OS process execution and stop/kill contracts. |
| `internal/platform/logging` | The framework-free `slog` handler: in-memory console, rolling session files, support export, redaction. |
| `internal/platform/dataroot` | Data-root pointer, relocation copy/finalize orchestration, total-size enumeration. |
| `internal/platform/credentials` | Native OS credential store (Secret Service/Windows Credential Manager) with no plaintext fallback, pending-commit reconciliation, legacy migration. |
| `internal/platform/filesystem` | Archive install, client-settings sanitize/clear, disk space, mod file layout. |
| `internal/platform/instancedirectory` | Instance directory allocation, clone storage, launch logs, log hardening. |
| `internal/platform/instancepackage` | The `.waxlight` archive format reader/writer and path-security policy. |
| `internal/platform/versionfs` | Version directory layout with collision-free unsafe-ID storage. |
| `internal/platform/gameversion` | Game archive installer for the version catalog. |
| `internal/platform/downloader` | HTTP download manager with concurrency and progress. |
| `internal/platform/modcatalog` | Thin `vintagestory-go` adapter for ModDB browsing. |
| `internal/platform/modstorage` | Downloaded-mod cache on disk. |
| `internal/platform/servercatalog` | Thin `vintagestory-go` adapter for the public server catalog. |
| `internal/platform/vintagestory` | Thin `vintagestory-go` adapters: auth client, version catalog. |
| `internal/platform/updater` | Update source (GitHub), installer, signature verifier, wait-for-parent. |
| `internal/platform/securefs` | Data-root permissions hardening. |
| `internal/platform/atomicfile` | Atomic file writes. |
| `internal/platform/nativefs` | Native directory opening. |
| `internal/platform/mousenavigation` | Navigation gesture helper (used by `cmd/waxlight/main.go`). |

## Dependency Rules

1. **Features are independent.** A feature imports other features only through
   ports and minimal structural views (for example `mods.InstanceRef`,
   `snapshots.InstanceRef`). No feature may import `internal/app`,
   `internal/transport/wails`, or `internal/platform/*`.
2. **Adapt to the outside.** SQLite, filesystem, OS, and network behavior lives
   in `internal/platform/*` adapters. Features define the ports; `wire.go`
   provides the implementations (including store-to-feature mappings in
   `internal/app/adapters.go`).
3. **Wails is a transport.** `internal/transport/wails` converts parameters and
   DTOs, calls features, and emits events. It never contains domain logic and
   never touches SQLite.
4. **The composition root wires everything.** `internal/app/wire.go` is the
   only place that constructs dependencies. No `NewXxx` is called from
   controllers, features, or adapters to build another layer's services.
5. **Errors flow through `internal/errs`.** Features and transport report
   user-facing failures with `errs.NewError`/`errs.AppError`; the code string
   is the frontend contract.

These rules are enforced by `scripts/check-architecture.sh` (see Validation).

## Lifecycle

1. `cmd/waxlight/main.go` installs the logger, waits for a parent updater
   process if present, then calls `app.New()` (or `app.NewWithHome` in tests).
2. `app.New` constructs, in order: the data root, the SQLite store and
   migrations, the session service (which recovers interrupted sessions), the
   lifecycle, the event adapter, the operations manager (which reconciles
   interrupted operations), settings, versions, telemetry, the instance
   creator, the snapshot/recovery services, the mods services, the instance
   mutation services, accounts, the launch coordinator, and every controller.
   Any failure closes the store and aborts startup.
3. Wails calls `Container.Startup`, which derives the lifecycle context from
   the framework context, installs the log emitter, and starts the
   non-blocking account validation and telemetry heartbeat.
4. `Container.Shutdown` detaches the log emitter, cancels the lifecycle
   context, joins every lifecycle-owned worker, records established launches,
   and closes the shared database.

## Events

| Event | Emitted by |
|---|---|
| `operation:created` / `updated` / `progress` / `completed` / `failed` / `removed` | `internal/operations` |
| `instance:created` / `updated` / `deleted` | `internal/instances` |
| `mod:installed` / `enabled` / `disabled` / `removed` / `linked` | `internal/mods` |
| `mods:task-progress` / `mods:downloads-changed` | `internal/mods` (ModDB task manager) |
| `favorite-server:updated` / `removed` | `internal/servers` |
| `game:started` / `game:exited` | `internal/launching` |
| `game:recovery-suggestion` / `last-known-good:updated` | `internal/recovery` |
| `data-folder:progress` / `data-folder:error` | `internal/settings` |
| `updates:progress` | `internal/transport/wails` (launcher update controller) |
| `logs:append` | `internal/app` (startup log emitter) |
| `navigation:mouse` | `cmd/waxlight/main.go` |

Event names and payload shapes are frontend contracts; keep them stable.

## Migrations

Runtime SQLite schema changes belong in
`internal/platform/sqlite/migrations.go`. Migrations are versioned,
transactional, and guarded against databases created by a newer launcher.
Files under `migrations/` are legacy artifacts, not canonical runtime
migrations.

## Security Boundaries

- Passwords, TOTP codes, session keys, and signatures live only in the native
  OS credential store (`internal/platform/credentials`). They must never
  appear in DTOs, bindings, logs, errors, process arguments, environment, or
  exports. At most they are written to the game's `clientsettings.json` for
  the duration of a running session and are guaranteed to be removed
  afterward.
- Any copy/export/backup of an instance removes `sessionkey`,
  `sessionsignature`, `playeruid`, and `playername` from `clientsettings.json`
  (`internal/platform/filesystem` sanitizer).
- All launcher logging goes through `internal/platform/logging`; never import
  stdlib `log`. Log only launcher events — never game output, credentials, or
  account data.
- User-facing errors use `internal/errs` codes; the code strings must not
  change because the frontend maps them to localized messages.
- Vintage Story protocol, catalog, parsing, and compatibility behavior must be
  contributed to `github.com/AmadoMuerte/vintagestory-go`, not re-implemented.

## Extension Checklist

1. **New feature** → create `internal/<feature>/` with models, ports,
   services, and tests. Consume other features through ports only.
2. **New controller method** → add it to the matching controller in
   `internal/transport/wails`, regenerate bindings (`make wails-build`),
   regenerate the API schema and inventory (`make api-docs`), and update the
   frontend `shared/api` module. The inventory, binding, and frontend checks
   run in `go test ./internal/transport/wails/`.
3. **New persisted repository** → implement it in
   `internal/platform/sqlite` with a versioned migration; wire it in
   `internal/app/wire.go`.
4. **New platform adapter** → create it under `internal/platform/<name>`,
   framework-free, importing features only for their models/ports.
5. **New data directory inside the data root** → update the dataroot
   relocation in `internal/platform/dataroot`: do not add it to
   `reservedNames`, and add it to the directory list in `removeOldRoot`.
6. **Wails dependencies** in features, platform imports in features, or a new
   god service are architecture violations and will fail
   `scripts/check-architecture.sh`.
