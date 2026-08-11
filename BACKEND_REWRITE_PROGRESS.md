# Backend Rewrite Progress

This document tracks the phased rewrite of the Waxlight Go backend into a
feature-oriented modular monolith with hexagonal boundaries.

The rewrite is intentionally incremental. Every stage must preserve the
existing frontend contract, database contents, credential guarantees, and
launcher behavior while leaving the repository buildable and testable.

## Current Status

- Base branch: `dev`
- Stage 1 pull request: [#81](https://github.com/AmadoMuerte/Waxlight-launcher/pull/81)
- Stage 1 status: merged into `dev`
- Stage 2 branch: `refactor/backend-core-features`
- Stage 2 pull request: [#82](https://github.com/AmadoMuerte/Waxlight-launcher/pull/82)
- Stage 2 status: implemented, validated locally, committed, synchronized, and
  submitted for review; CI and merge are pending
- Overall rewrite status: in progress

The final acceptance criteria are not met yet. In particular,
`internal/application.Service`, the remaining broad application ports, and the
old presentation/infrastructure package hierarchy still exist for features
that have not been migrated.

## Stage 1: Foundation and Accounts

Stage 1 established boundaries that later features can use without changing
frontend behavior.

### SQLite Boundary

- Removed the 1,300-line `internal/infrastructure/database/sqlite.go`.
- Added `internal/platform/sqlite` with one shared `*sql.DB` connection.
- Split persistence implementation by responsibility:
  - accounts;
  - versions;
  - instances;
  - servers;
  - mods;
  - sessions;
  - operations;
  - settings;
  - recovery metadata.
- Kept the existing database location, table names, columns, and stored data.
- Separated connection setup from migration execution and repository queries.
- Made migrations transactional and version-aware.
- Added protection against opening databases created by a newer unsupported
  schema version.
- Added migration coverage for legacy account columns, account UID indexes,
  operation localization columns, transaction rollback, and existing data.
- Preserved data-root path relocation for all persisted absolute paths.

### Accounts Feature

- Added `internal/accounts` as the owner of:
  - account models and statuses;
  - login and TOTP results;
  - authentication session mapping;
  - credential ports;
  - account repository interface;
  - login-flow state and validation;
  - account persistence and rollback behavior;
  - account tests.
- Removed account models, auth types, credential interfaces, and account
  service implementation from `internal/domain` and `internal/application`.
- Replaced the broad store dependency with a six-method account repository.
- Made account dependencies immutable at construction.
- Removed account `ConfigureInstanceCleanup` and `ConfigureTelemetry` methods.
- Preserved login, reauthentication, TOTP, stale-session validation, default
  account selection, logout, removal, and rollback behavior.
- Preserved the native OS credential store with no plaintext or in-memory
  production fallback.
- Kept passwords, TOTP codes, pre-login tokens, session keys, and signatures
  out of SQLite, logs, DTOs, and generated bindings.
- Kept interrupted credential-commit reconciliation and legacy secret
  migration behavior.

### Lifecycle and Events

- Added `internal/app.Lifecycle` as the application context owner.
- Wails Startup now supplies the parent application context.
- Added lifecycle-owned worker cancellation and deterministic waiting during
  shutdown.
- Added the framework-neutral `internal/events.Publisher` abstraction.
- Added the Wails event adapter under `internal/transport/wails`.
- Removed the presentation `Base` event/context holder.
- Routed launcher events and live logs through the Wails transport adapter.
- Updated controllers to use the lifecycle context instead of creating
  unrelated background contexts where practical.
- Removed the accidentally generated browser-callable `AppController.Startup`
  binding while preserving all frontend feature APIs.
- Kept stale-account validation from blocking shutdown on an uninterruptible
  native keyring call.

### Stage 1 Validation

The following checks passed before PR #81 was opened:

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `make format-check`
- `make lint`
- `make security`
- `npm test --prefix frontend`
- `npm run build --prefix frontend`
- `make wails-build` on Linux AMD64

PR #81 also passed the repository CI jobs for tests/static checks, security,
Linux and Windows native credential stores, and Linux and Windows production
builds.

## Stage 2: Operations, Versions, and Settings

Stage 2 is implemented on `refactor/backend-core-features` and delivered for
review in pull request #82.

### Operations Feature

- Added `internal/operations` as the owner of the operation model, statuses,
  event names, focused repository interface, runtime workers, cancellation,
  and futures.
- Removed `domain.Operation`.
- Removed the following global state from `application.Service`:
  - `operationsMu`;
  - `operationCancels`;
  - `operationDone`;
  - `versionOperations`;
  - `operationWG`;
  - the service-owned shutdown context and cancel function.
- Centralized operation persistence and these existing UI events:
  - `operation:created`;
  - `operation:updated`;
  - `operation:progress`;
  - `operation:completed`;
  - `operation:failed`;
  - `operation:removed`.
- Added typed lifecycle-owned workers and stable futures.
- Fixed a race where package import could miss a fast version-install
  completion.
- Fixed worker error propagation so waiting callers receive installation
  failures instead of treating them as success.
- Separated persistent operation cancellation from ephemeral ModDB task
  cancellation, preventing a ModDB task ID from blocking `CancelOperation` on
  a nil completion channel.
- Preserved the current behavior where a cleanly cancelled version download is
  removed from operation history and cleanup failures remain visible as failed
  operations.
- Added startup recovery for persisted `queued` or `running` operations left by
  a crash. They become terminal interrupted failures with a user-facing message
  and finish timestamp.
- Ensured recovered operations can be listed, deleted, and cleared and no
  longer block data-root relocation forever.

### Download Contract

- Added `internal/downloads` for transport-neutral download request, progress,
  and downloader contracts.
- Removed the downloader adapter's dependency on `internal/application`.
- Updated game-version, mod, and launcher-update callers to use the neutral
  contract.

### Versions Feature

- Added `internal/versions` as the owner of installed and available game-version
  models.
- Removed `domain.GameVersion`, `domain.AvailableGameVersion`, and
  `internal/application/versions.go`.
- Added feature-owned repository, catalog, downloader, installer, disk-space,
  mutation, filesystem, operation, and event boundaries.
- Replaced the broad version service with focused capabilities for:
  - installed/catalog queries and executable repair;
  - local archive installation;
  - official catalog download and installation;
  - version removal.
- Removed `ConfigureVersionDownloads` and `ConfigureDiskSpaceChecker`.
- Updated the GameVersion Wails controller to call version capabilities
  directly while retaining existing method names, arguments, DTO fields, and
  error behavior.
- Updated instances, launching, mods, snapshots, recovery, package import,
  support logs, SQLite, and Vintage Story adapters to use version-owned models.
- Kept `vintagestory-go v0.1.0` as the source of truth for the official version
  catalog.
- Made local and catalog installations of the same version share one operation
  resource claim so they cannot race each other.
- Kept package import waiting on the operation subsystem rather than reading
  internal channels.
- Added catalog filename validation to prevent path traversal.
- Added collision-free storage paths for unsafe version IDs while preserving
  normal safe IDs.
- Added version-ID length and control-character validation.
- Made managed version removal verify that `.waxlight-version` contains the
  exact expected version ID before deleting files.
- Added tests for colliding unsafe IDs, concurrent install claims, invalid IDs,
  marker mismatch, catalog traversal, cancellation, and existing install/remove
  behavior.
- Made cancellation remove partial local and catalog installation targets while
  preserving `context.Canceled` for waiting callers.
- Made marker and repository failures after extraction remove the owned target
  instead of leaving an untracked game installation.

### Settings Feature

- Added `internal/settings` as the owner of:
  - launcher settings model and defaults;
  - data-folder and relocation progress models;
  - settings repository and value-repository ports;
  - normalized settings reader;
  - settings update service;
  - data-root relocation coordinator.
- Removed `domain.Settings`.
- Removed settings and generic key/value methods from `application.Service` and
  the broad application store.
- Made telemetry consume the settings reader and value repository directly.
- Preserved language normalization, update-channel validation, skipped-version
  validation, telemetry consent synchronization, heartbeat-on-enable behavior,
  and runtime download concurrency changes.
- Moved settings defaults from SQLite policy into the settings feature while
  preserving legacy JSON behavior.
- Moved data-root relocation orchestration out of the Wails controller.
- Moved native dialogs and quit behavior to `internal/transport/wails`.
- Moved native directory opening to a focused infrastructure adapter.
- Kept all existing SettingsController methods and DTO/event payloads.

### Mutation and Data-Root Safety

- Added `internal/mutations.Gate` to atomically coordinate launcher writes and
  data-root relocation.
- Relocation cannot begin while a tracked mutation is active.
- New mutations cannot begin after relocation has acquired the gate.
- Gated account/client-settings cleanup, instance writes, cloning, package
  import/export work, mod changes/download persistence, snapshots, launching,
  and version operations for their complete mutation duration.
- Preserved all launcher-owned relocation directories:
  - `versions`;
  - `instances`;
  - `downloads`;
  - `cache`;
  - `security`;
  - `updates`;
  - `logs`;
  - `backups`.
- Made relocation work lifecycle-owned and cancellation-aware.
- Made initial data-size enumeration context-aware so shutdown does not wait
  indefinitely before copying starts.
- Synchronized progress throttling because copy callbacks can run concurrently.
- Made relocation acquisition atomic instead of separating the busy check from
  the state update.
- Kept the relocation gate held through successful relaunch/quit.
- On copy or relaunch failure, release the gate, retain a user-visible error,
  clear pending state, clean only copied target data, preserve a pre-existing
  target directory, and permit retry.
- Added tests for concurrent relocation, mutation races, cancellation during
  enumeration/copying, progress events, relaunch rollback, retry, and every
  persistent launcher directory.
- Made relocation copy into an exclusively owned staging directory and defer
  the final target rename until startup, preserving unrelated target data.
- Added explicit committed-copy recovery and unique ownership markers so
  crashes during finalization can resume without orphaning copied data.
- Kept native credential-store and remote account validation outside the
  mutation gate while retaining the gate around metadata writes.

### Stage 2 Validation

The final combined review found no remaining actionable issues and confirmed
that the Wails/frontend API, DTO, and event contracts remain compatible. On
2026-08-11, these checks passed:

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `make format-check`
- `make lint`
- `make security`
- `npm test --prefix frontend` (27 files, 149 tests)
- `npm run build --prefix frontend`
- `make wails-build` on Linux AMD64
- `git diff --check`

Native Windows credential-store integration and Windows production build checks
remain CI-only.

Manual smoke testing also passed for the implemented Stage 2 behavior.

## Remaining Work

### Finish and Deliver Stage 2

- [x] Perform the final combined Stage 2 review.
- [x] Confirm Wails/frontend API, DTO, and event compatibility.
- [x] Run the complete local validation matrix.
- [x] Complete manual smoke testing of the implemented behavior.
- [x] Commit Stage 2 with a clear English commit message.
- [x] Synchronize `refactor/backend-core-features` with the latest `origin/dev`.
- [x] Push the branch and open pull request #82 against `dev`.
- [ ] Wait for CI before starting the next integration stage.

### Instances, Launching, and Sessions

- Move the instance model and repository interface into `internal/instances`.
- Split instance CRUD, clone, share/import/export, local storage, and update
  analysis into coherent capabilities.
- Remove instance methods and synchronization from `application.Service`.
- Add a focused launch coordinator that separates:
  - instance resolution;
  - account/session resolution;
  - client-settings preparation;
  - process argument construction;
  - process start/stop/tracking;
  - launcher-side logs;
  - sensitive settings restoration;
  - play-session recording;
  - crash/recovery handling.
- Move process execution behind a platform adapter.
- Make all launch workers lifecycle-owned and joined during shutdown.
- Move play-session models/repository and statistics inputs to feature-owned
  packages.
- Preserve credential injection cleanup and startup reconciliation.

### Servers

- Move favorite/public server models and repository interface into
  `internal/servers`.
- Keep the public catalog behind the `vintagestory-go` server adapter.
- Remove `ConfigurePublicServerCatalog`.
- Move the Wails server controller to the new transport boundary.
- Preserve favorite server storage and launch behavior.

### Mods and Mod Catalog

- Move installed/downloaded mod models, repository ports, file-management
  boundaries, download coordination, updates, enable/disable, removal, local
  linking, and dependency installation into focused mod capabilities.
- Separate ModDB browsing from installed-mod orchestration.
- Move ModDB task tracking to a focused manager while retaining current event
  names and frontend behavior.
- Remove `ConfigureMods` and remaining mod state from `application.Service`.
- Keep ModDB and modpack behavior sourced from `vintagestory-go`.
- Resolve the remaining duplicate local `modinfo.json` parsing and generic
  dependency compatibility behavior by contributing missing generic capability
  upstream instead of maintaining duplicate Waxlight implementations.

### Snapshots and Recovery

- Move snapshots, safety snapshots, retention, restore coordination, and
  snapshot models into `internal/snapshots`.
- Move Last Known Good and crash-recovery behavior into a focused recovery
  package.
- Define narrow snapshot filesystem and repository ports.
- Remove direct application imports of snapshot/data-root/filesystem adapters.
- Preserve manual snapshots, safety snapshots, automatic retention, exact
  managed-mod restoration, recovery suggestions, and credential sanitization.
- Add direct snapshot-store integration tests in addition to feature tests.

### Updater, Telemetry, Statistics, and Game Logs

- Move launcher update orchestration into `internal/updates` with immutable
  telemetry and platform dependencies.
- Remove `LauncherUpdateService.ConfigureTelemetry`.
- Keep trusted URL, checksum, signature, publisher, and installer guarantees.
- Move telemetry orchestration to the final feature/platform structure without
  weakening opt-in or payload allowlists.
- Move statistics and playtime queries into `internal/statistics`.
- Move game-log tailing/crash-report coordination into `internal/gamelog`.
- Ensure telemetry and background delivery have explicit lifecycle ownership.

### Wails Transport and Composition Root

- Move every controller from `internal/presentation` into
  `internal/transport/wails`, split by feature.
- Keep Wails limited to parameter/DTO conversion, feature invocation, dialogs,
  events, browser opening, and application quit.
- Preserve all frontend-consumed controller names, method names, arguments,
  return shapes, events, and user-facing errors.
- Add an automated frontend-to-backend Wails contract check for the complete API
  inventory.
- Move bootstrap wiring into `internal/app/wire.go` and lifecycle ownership into
  `internal/app`.
- Keep `cmd/waxlight/main.go` as a small executable entrypoint.
- Remove `internal/bootstrap` after the composition root is complete.

### Remove the Legacy Architecture

- Delete `internal/application.Service` after all callers are migrated.
- Delete the remaining broad `application.Store` and unrelated global ports.
- Remove all remaining post-construction configuration methods:
  - `ConfigureAuthentication`;
  - `ConfigureMods`;
  - `ConfigurePublicServerCatalog`;
  - `ConfigureTelemetry`;
  - `SetEventPublisher`.
- Delete obsolete application feature files, shared global models, controller
  monoliths, adapters, compatibility helpers, and dead tests.
- Move remaining infrastructure adapters under `internal/platform` by focused
  responsibility.
- Remove the old global `internal/domain`, `internal/application`,
  `internal/infrastructure`, and `internal/presentation` hierarchy once no
  feature depends on it.
- Ensure no new service, module, repository, or dependency container replaces
  the old god objects.

### Architecture Documentation and Enforcement

- Add `docs/backend-architecture.md` describing the final implemented package
  graph, module ownership, dependency direction, lifecycle, events, migrations,
  security boundaries, and extension rules.
- Update `AGENTS.md` to require the feature-oriented architecture rather than
  the old global layered structure.
- Add architecture checks that reject:
  - Wails imports from features;
  - SQLite/platform imports from features;
  - credentials in DTOs/generated bindings;
  - forbidden global service/store patterns;
  - duplicate generic Vintage Story implementations.
- Add a complete Wails API compatibility inventory/check.

### Final Validation and Delivery

Before the rewrite is considered complete, run and report:

```bash
gofmt
go test ./...
go test -race ./...
go vet ./...
make format-check
make lint
make security
npm test --prefix frontend
npm run build --prefix frontend
make wails-build
```

Platform-specific checks that cannot run locally must be reported explicitly
and covered by CI. The final branch must be pushed and the final normal pull
request must target `dev`; it must not be merged automatically.
