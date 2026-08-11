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
- Stage 2 status: merged into `dev` after successful local validation, manual
  smoke testing, and CI
- Stage 3 initial branch: `refactor/backend-instances-launching`
- Stage 3 initial pull request: [#84](https://github.com/AmadoMuerte/Waxlight-launcher/pull/84)
- Stage 3 initial status: merged into `dev` after successful local validation,
  manual smoke testing, and CI
- Current Stage 3 branch: `refactor/backend-instance-cloning`
- Current Stage 3 pull request: [#85](https://github.com/AmadoMuerte/Waxlight-launcher/pull/85)
- Stage 3 status: in progress; play-session, instance-core, CRUD,
  local-storage, and cloning ownership extracted
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

Stage 2 was implemented on `refactor/backend-core-features` and merged into
`dev` through pull request #82.

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

PR #82 also passed CI tests/static checks, vulnerability and secret scanning,
Linux and Windows native credential-store integration, and Linux and Windows
production builds.

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
- [x] Pass all required CI checks.
- [x] Merge pull request #82 into `dev`.

## Stage 3: Instances, Launching, and Sessions

Goal: remove instance and game-process orchestration from
`internal/application.Service` without changing instance storage, launch
arguments, credential handling, or frontend behavior.

### Completed Stage 3 Slice: Sessions

- Added `internal/sessions` as the owner of persisted play-session and
  statistics models.
- Added a narrow session repository plus focused create, finish, interrupted
  recovery, statistics, and per-instance playtime capabilities.
- Removed play-session models and persistence methods from `internal/domain`
  and the broad `application.Store`.
- Made session completion and startup recovery timestamps feature-owned and
  injectable for deterministic tests.
- Made interrupted-session and running-instance recovery transactional instead
  of ignoring a partial recovery failure.
- Replaced the 5,000-row statistics cap with aggregate SQLite queries while
  retaining a separate ten-session recent-history limit.
- Updated launch orchestration to use the session capability while keeping
  process, credential-cleanup, and game-exit ordering together until the
  launching feature is extracted.
- Updated bootstrap, statistics presentation, instance playtime aggregation,
  DTO mapping, and SQLite mappings without changing frontend contracts or the
  database schema.
- Added direct feature tests and SQLite tests for round trips, finish state,
  interrupted-session recovery, statistics, and playtime.
- Passed `go test ./...`, focused race tests for sessions, SQLite, application,
  presentation, and bootstrap, `make format-check lint`, and `make security`.

### Completed Stage 3 Slice: Instance Core

- Added `internal/instances` as the owner of the instance model, create input,
  ready/running statuses, instance error codes, repository, and query service.
- Removed `domain.Instance`, the application-owned create input, and the
  instance error codes from `internal/domain` without compatibility aliases.
- Made the broad application store compose the focused instance repository
  instead of redeclaring its methods.
- Moved SQLite instance mapping to `instances.Instance` without changing the
  schema, columns, ordering, JSON launch arguments, nullable fields, or error
  messages.
- Updated application orchestration, telemetry, presentation DTOs, bootstrap,
  and tests to consume the instance-owned model.
- Made `InstanceController` use the focused query capability for list/get while
  retaining existing Wails method names and DTO shapes.
- Added direct query-service and SQLite tests for CRUD, mapping, ordering,
  directory conflicts, and missing records.
- Passed `go test ./...`, focused race tests for instances, sessions, SQLite,
  application, presentation, telemetry, and bootstrap, all frontend tests, and
  the frontend production build. Format, lint, security, and vulnerability
  checks also passed.

### Completed Stage 3 Slice: Instance Creation and Local Storage

- Added an instance-owned creation service with immutable, focused repository,
  version, account, settings-language, mutation-gate, directory-storage, event,
  telemetry, clock, and ID-generation dependencies.
- Split instance query and creation repository boundaries so read-only
  capabilities no longer depend on the complete instance repository.
- Moved name validation, localized default naming, suffix allocation, version
  and account validation, timestamps, persistence, creation events, and
  creation telemetry out of `application.Service`.
- Added an instance-directory infrastructure adapter that normalizes paths,
  serializes in-process allocation, uses an exclusive instance marker as a
  cross-adapter reservation, creates the standard mod/log layout, hardens logs,
  and preserves the exact marker contents and permissions.
- Made failed creation roll back only allocation-owned files. A newly created
  instance root is removed, while a pre-existing custom directory and its user
  content are preserved.
- Closed the custom-directory check/allocate race for concurrent creators and
  prevented a losing allocator from deleting the winning allocator's files.
- Made `InstanceController.CreateInstance` call the focused creation capability
  directly while preserving its Wails method, request, response, error, event,
  and telemetry contracts.
- Kept a narrow application delegate for clone and package-import orchestration
  until those capabilities move into the instance feature.
- Added direct creation and directory-adapter tests for naming, validation,
  mutation gating, events, telemetry, marker/layout creation, conflicts,
  persistence rollback, pre-existing directory preservation, symlink roots,
  and concurrent independent allocators.
- Passed focused tests and race tests for instances, instance-directory
  storage, application, presentation, and bootstrap.

### Completed Stage 3 Slice: Instance Update and Deletion

- Added instance-owned update and deletion services with focused repository,
  version, mutation-gate, client-settings, safety-snapshot, process-state,
  recovery-cleanup, filesystem, event, telemetry, and clock boundaries.
- Moved update validation, persisted-field preservation, account-change
  credential cleanup, timestamps, persistence, and update events out of
  `application.Service`.
- Kept game-version safety snapshots behind a narrow transitional callback so
  the per-instance mutation lock remains held from snapshot creation through
  persistence without making `internal/instances` depend on the legacy
  snapshot subsystem.
- Moved deletion sequencing, optional client-settings cleanup, repository
  deletion, best-effort Last Known Good cleanup, events, and telemetry out of
  `application.Service`.
- Preserved running-game and snapshot-operation deletion guards through a
  narrow application adapter until process and snapshot ownership moves in
  later Stage 3 and Stage 6 slices.
- Made `InstanceController` call update and deletion capabilities directly and
  removed the migrated `UpdateInstance` and `DeleteInstance` orchestration
  methods from `application.Service` without changing Wails methods or DTOs.
- Added direct tests for field preservation, timestamps, version-change lock
  ordering, credential cleanup, mutation-gate rejection, filesystem/persistence
  ordering, best-effort recovery cleanup, events, telemetry, and failure
  rollback boundaries.
- Passed `go test ./...` and focused race tests for instances, application,
  presentation, bootstrap, and SQLite.

### Completed Stage 3 Slice: Instance Cloning

- Added an instance-owned clone service with focused instance, mod, creation,
  mutation-gate, source-guard, filesystem, client-settings, cleanup, clock, and
  ID-generation boundaries.
- Moved clone metadata orchestration, mod-row replication, path remapping,
  cover remapping, final persistence, and rollback out of
  `application.Service`.
- Added an instance-directory clone adapter that preserves the new clone marker,
  copies regular instance files and modes, skips saves, logs, source markers,
  and authentication journals, rejects symbolic links and non-regular files,
  verifies opened source files, refuses target replacement, and honors
  cancellation during file copies.
- Made `clientsettings.json` sanitize in memory before its first destination
  write, removing authentication, account, server-history, and machine-path
  data without modifying the source or creating a crash window with copied
  credentials.
- Preserved complete mutation-gate coverage and made cloning reserve the source
  instance against launches, snapshots, and destructive mutations until the
  clone completes.
- Made manual snapshot creation and restore reserve the same per-instance slot
  atomically with launch checks, and extended that slot to local/catalog mod
  installation, enable/disable, and local-catalog linking so clone inputs
  cannot change midway through copying.
- Made instance deletion reserve the same slot through filesystem and database
  cleanup, preventing deletion from racing clone or snapshot work.
- Preserved creation event semantics, mod metadata, rollback ordering, and
  primary-error propagation when cleanup also fails; cancellation now uses a
  bounded live cleanup context, and a failed directory cleanup retains the
  instance record as recovery metadata instead of orphaning files.
- Made `InstanceController.CloneInstance` call the focused clone capability
  directly and deleted the legacy clone implementation from
  `internal/application` without changing the Wails request or response.
- Added direct clone-service and filesystem-adapter tests for metadata, launch
  argument isolation, mod and cover paths, operation ordering, rollback,
  excluded content, sanitize-before-write behavior, marker preservation,
  symlink and overlapping-root rejection, cancellation, mutually exclusive
  source/snapshot reservations, live cleanup context, and internal-path
  mapping.

### Instance Feature

- [x] Add `internal/instances` as the owner of the instance model, inputs,
  repository interfaces, statuses, and instance-specific errors.
- [ ] Split instance behavior into focused capabilities for:
  - queries and CRUD (complete);
  - directory allocation and local storage (creation allocation extracted;
    deletion and restore storage remain);
  - cloning (complete);
  - package import and export;
  - update analysis and game-version changes.
- [ ] Replace the broad application store dependency with narrow instance,
  version, account, mod, snapshot, and filesystem ports.
- [ ] Move instance mutation locking into the feature and integrate it with
  `internal/mutations.Gate`.
- [x] Move instance SQLite mappings to instance-owned models while preserving
  the current schema and stored rows.
- [ ] Remove migrated instance methods, state, and tests from
  `internal/application.Service`.

### Launching and Processes

- [ ] Add `internal/launching` with a focused launch coordinator that separates:
  - instance and version resolution;
  - account and session validation;
  - client-settings preparation and cleanup;
  - process argument and environment construction;
  - process start, stop, tracking, and exit handling;
  - launcher-owned diagnostic logs;
  - play-session persistence;
  - failed-startup and crash handling.
- [ ] Move process execution and OS-specific stop/kill behavior under
  `internal/platform/process`.
- [ ] Make every launch worker lifecycle-owned and joined during shutdown.
- [ ] Preserve temporary credential injection into `clientsettings.json` and
  guaranteed cleanup after normal exit, failed launch, forced stop, and startup
  reconciliation.
- [ ] Keep credentials out of process arguments, environment variables, logs,
  DTOs, errors, and events.
- [ ] Preserve current launch validation messages, server-connect behavior,
  game events, and frontend controller contracts.

### Sessions and Statistics Inputs

- [x] Add `internal/sessions` as the owner of play-session models and repository
  interfaces.
- [x] Move session start/finish recording and interrupted-session recovery out
  of `application.Service`.
- [x] Expose narrow read capabilities that a later statistics feature can use.
- [x] Preserve playtime, crash, exit-code, process-ID, and account associations.

### Stage 3 Validation and Delivery

- [ ] Add direct tests for instance CRUD, clone, package operations, validation,
  launch rollback, credential cleanup, process failures, shutdown, and session
  recovery.
- [ ] Run focused race tests for instances, launching, sessions, bootstrap,
  SQLite, and presentation.
- [ ] Confirm all instance/game Wails methods, DTOs, events, and error messages
  remain compatible.
- [ ] Run the complete local validation matrix.
- [ ] Complete manual smoke testing for create/edit/clone/import/export,
  authenticated and offline launch, stop, crash, and restart recovery.
- [ ] Commit, synchronize with `origin/dev`, push, open a pull request against
  `dev`, pass CI, and merge before Stage 4 begins.

## Stage 4: Servers and Public Catalog

Goal: isolate favorite-server persistence and public-server browsing while
keeping Vintage Story protocol behavior in `vintagestory-go`.

### Server Feature

- [ ] Add `internal/servers` as the owner of favorite/public server models,
  validation, repository ports, and launch requests.
- [ ] Split favorite-server CRUD from public-catalog queries.
- [ ] Keep public catalog access behind a thin `vintagestory-go` adapter.
- [ ] Remove `ConfigurePublicServerCatalog` and inject immutable dependencies at
  construction.
- [ ] Move the server controller into `internal/transport/wails` while
  preserving controller and method names.
- [ ] Preserve favorite-server rows, address validation, instance association,
  public catalog fields, and connect-on-launch behavior.

### Stage 4 Validation and Delivery

- [ ] Add tests for favorite-server CRUD, validation, catalog mapping, catalog
  failures, and server launches.
- [ ] Confirm server DTOs, generated bindings, events, and frontend calls remain
  compatible.
- [ ] Run focused race tests and the complete local validation matrix.
- [ ] Complete manual smoke testing for favorites, public browsing, refresh,
  errors, and joining a server.
- [ ] Commit, synchronize, push, open a pull request against `dev`, pass CI, and
  merge before Stage 5 begins.

## Stage 5: Mods and ModDB

Goal: separate installed-mod orchestration from ModDB browsing and eliminate
the remaining mod state in `application.Service`.

### Upstream Vintage Story Work

- [ ] Inventory remaining local `modinfo.json`, dependency, compatibility, and
  modpack behavior against the released `vintagestory-go` API.
- [ ] Contribute missing generic parsing or compatibility capabilities to
  `vintagestory-go` with upstream tests.
- [ ] Release and consume the required library version before deleting any
  temporary local generic implementation.
- [ ] Keep only Waxlight-specific mapping, persistence, filesystem policy,
  orchestration, and user-facing errors in this repository.

### Installed Mods

- [ ] Add `internal/mods` as the owner of installed/downloaded mod models,
  repository ports, mutation coordination, and feature errors.
- [ ] Split capabilities for:
  - installed-mod discovery and reconciliation;
  - local file installation and linking;
  - enable/disable and removal;
  - dependency installation and removal previews;
  - downloaded-mod cache and cleanup;
  - update analysis and application.
- [ ] Define narrow filesystem, downloader, catalog, snapshot, instance, and
  event boundaries.
- [ ] Preserve safety snapshots before destructive mod changes.
- [ ] Remove migrated mod methods, mutexes, task maps, and state from
  `application.Service`.

### ModDB Browsing and Tasks

- [ ] Add a focused ModDB catalog capability backed by `vintagestory-go`.
- [ ] Move ModDB task tracking, cancellation, progress, and completion into a
  dedicated manager separate from persistent launcher operations.
- [ ] Remove `ConfigureMods` and use immutable constructor dependencies.
- [ ] Preserve current ModDB query behavior, pagination, dependency flow,
  event names, DTOs, and frontend task cancellation.

### Stage 5 Validation and Delivery

- [ ] Add tests for discovery, reconciliation, local linking, dependencies,
  updates, cancellation, cache cleanup, unsafe archives, and rollback.
- [ ] Run focused race tests for mods, ModDB tasks, downloader, snapshots,
  SQLite, bootstrap, and presentation.
- [ ] Confirm all mod Wails methods, DTOs, events, and generated bindings remain
  compatible.
- [ ] Run the complete local validation matrix and manual mod-management smoke
  tests.
- [ ] Commit, synchronize, push, open a pull request against `dev`, pass CI, and
  merge before Stage 6 begins.

## Stage 6: Snapshots and Recovery

Goal: isolate backup, restore, Last Known Good, and crash-recovery policy behind
narrow repositories and filesystem boundaries.

### Snapshot Feature

- [ ] Add `internal/snapshots` as the owner of snapshot models, reasons,
  retention policy, repository ports, and capabilities.
- [ ] Move manual snapshots, safety snapshots, restore coordination, pruning,
  and exact managed-mod restoration out of `application.Service`.
- [ ] Define narrow snapshot filesystem, instance, mod, version, and mutation
  ports.
- [ ] Move the snapshot storage adapter under `internal/platform/snapshots`.
- [ ] Ensure every snapshot, export, and archive path removes `sessionkey`,
  `sessionsignature`, `playeruid`, and `playername` from `clientsettings.json`.

### Recovery Feature

- [ ] Add `internal/recovery` for Last Known Good state, startup reconciliation,
  failed-launch analysis, recovery suggestions, and restore coordination.
- [ ] Preserve current crash-window behavior and user-facing recovery events.
- [ ] Remove direct application imports of snapshot, data-root, and filesystem
  adapters.
- [ ] Remove migrated snapshot/recovery methods and state from
  `application.Service`.

### Stage 6 Validation and Delivery

- [ ] Add direct integration tests for the snapshot storage adapter.
- [ ] Add tests for manual/safety snapshots, retention, exact restore,
  credential sanitization, failed restores, Last Known Good, and crash recovery.
- [ ] Run focused race tests and the complete local validation matrix.
- [ ] Complete manual smoke testing for create, restore, prune, failed launch,
  and suggested recovery.
- [ ] Commit, synchronize, push, open a pull request against `dev`, pass CI, and
  merge before Stage 7 begins.

## Stage 7: Updates, Telemetry, Statistics, and Game Logs

Goal: give the remaining operational services explicit feature ownership and
lifecycle-safe background delivery.

### Launcher Updates

- [ ] Add `internal/updates` for update checks, download orchestration,
  verification, installation, skip policy, and update events.
- [ ] Make telemetry, downloader, platform, publisher, and installer
  dependencies immutable.
- [ ] Remove `LauncherUpdateService.ConfigureTelemetry`.
- [ ] Preserve trusted URL validation, checksums, signatures, publisher trust,
  channel behavior, skipped versions, and installer guarantees.

### Telemetry

- [ ] Move telemetry orchestration and background delivery into the final
  feature/platform structure.
- [ ] Keep telemetry disabled until explicit opt-in and preserve event/error
  allowlists, identity privacy, consent synchronization, and heartbeat policy.
- [ ] Make every telemetry worker lifecycle-owned, cancellable where possible,
  and joined or safely abandoned during shutdown.
- [ ] Remove remaining post-construction telemetry configuration.

### Statistics and Game Logs

- [ ] Add `internal/statistics` for launcher statistics and per-instance
  playtime queries over the sessions read capability.
- [ ] Add `internal/gamelog` for game-log tailing, crash indicators, and
  launcher-side crash-report coordination.
- [ ] Preserve lazy frontend loading of the log console and existing events.
- [ ] Never copy game output into launcher logs or support exports.

### Stage 7 Validation and Delivery

- [ ] Add tests for update trust failures, cancellation, telemetry opt-in and
  allowlists, shutdown, statistics, log tailing, and crash coordination.
- [ ] Run focused race, privacy, security, and complete local validation checks.
- [ ] Complete manual smoke testing for update UI, telemetry transitions,
  statistics, and game-log viewing.
- [ ] Commit, synchronize, push, open a pull request against `dev`, pass CI, and
  merge before Stage 8 begins.

## Stage 8: Wails Transport and Composition Root

Goal: reduce Wails to a transport adapter and make `internal/app` the complete,
explicit composition root.

### Wails Transport

- [ ] Move every remaining controller and DTO from `internal/presentation` into
  feature-oriented files under `internal/transport/wails`.
- [ ] Keep Wails limited to parameter/DTO conversion, feature invocation,
  dialogs, events, browser opening, directory opening, and application quit.
- [ ] Preserve all frontend-consumed controller names, method names, argument
  order, return shapes, JSON fields, events, and user-facing errors.
- [ ] Regenerate bindings and verify that only intentional package-path changes
  occur.
- [ ] Add a checked-in Wails API inventory and an automated frontend-to-backend
  compatibility check.

### Composition Root

- [ ] Move dependency construction and wiring into `internal/app/wire.go`.
- [ ] Keep lifecycle, mutation gate, event publisher, repositories, features,
  platform adapters, and transport assembly explicit and immutable.
- [ ] Keep `cmd/waxlight/main.go` as a small executable entrypoint.
- [ ] Remove `internal/bootstrap` after all startup, reconciliation, and shutdown
  responsibilities have moved.
- [ ] Add composition tests that prove startup ordering, recovery ordering, and
  deterministic shutdown.

### Stage 8 Validation and Delivery

- [ ] Verify the complete Wails API inventory against frontend consumers and
  generated bindings.
- [ ] Run focused lifecycle, bootstrap/composition, transport, and race tests.
- [ ] Run the complete local validation matrix and a desktop smoke test.
- [ ] Commit, synchronize, push, open a pull request against `dev`, pass CI, and
  merge before Stage 9 begins.

## Stage 9: Legacy Removal and Architecture Enforcement

Goal: delete the old global architecture, document the implemented package
graph, and prevent architectural regression.

### Remove Legacy Packages

- [ ] Delete `internal/application.Service` after all callers are migrated.
- [ ] Delete the remaining broad `application.Store` and unrelated global
  ports.
- [ ] Remove all remaining post-construction configuration methods:
  - `ConfigureAuthentication`;
  - `ConfigureMods`;
  - `ConfigurePublicServerCatalog`;
  - `ConfigureTelemetry`;
  - `SetEventPublisher`.
- [ ] Delete obsolete application feature files, shared global models,
  controller monoliths, adapters, compatibility helpers, and dead tests.
- [ ] Move remaining infrastructure adapters under `internal/platform` by
  focused responsibility.
- [ ] Remove the old global `internal/domain`, `internal/application`,
  `internal/infrastructure`, and `internal/presentation` hierarchy once no
  feature depends on it.
- [ ] Confirm that no replacement god service, store, module, or dependency
  container has been introduced.

### Documentation and Enforcement

- [ ] Add `docs/backend-architecture.md` describing the implemented package
  graph, module ownership, dependency direction, lifecycle, events, migrations,
  security boundaries, and extension rules.
- [ ] Update `AGENTS.md` to require the final feature-oriented architecture.
- [ ] Add architecture checks that reject:
  - Wails imports from feature packages;
  - SQLite/platform imports from feature packages;
  - credentials in DTOs or generated bindings;
  - forbidden global service/store patterns;
  - duplicate generic Vintage Story implementations.
- [ ] Integrate architecture and Wails contract checks into local validation and
  CI.
- [ ] Update this progress document with the final package inventory and mark
  every rewrite stage complete.

### Stage 9 Validation and Delivery

- [ ] Run and report the complete final validation matrix:

```bash
make format
git diff --check
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

- [ ] Complete final manual testing for accounts, versions, instances, launch,
  servers, mods, snapshots, recovery, relocation, updates, statistics, and logs.
- [ ] Report any platform-specific checks that cannot run locally and require
  them in CI.
- [ ] Confirm the final branch is synchronized with `origin/dev` and contains
  only the intended rewrite changes.
- [ ] Push the final branch and open the final normal pull request against
  `dev`; do not target `main` and do not merge automatically.
- [ ] Pass every required CI check and complete review before merge.
- [ ] After merge, mark the backend rewrite complete and begin any release
  promotion only through the separate `dev` to `main` process.
