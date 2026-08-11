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
- Current Stage 3 branch: `refactor/backend-stage3-package-launch`
- Current Stage 3 pull request: [#86](https://github.com/AmadoMuerte/Waxlight-launcher/pull/86)
- Stage 3 status: complete; all instance, package, launching, process, and
  session ownership extracted from `application.Service`
- Stage 4 branch: `refactor/backend-servers-public-catalog`
- Stage 4 pull request: [#87](https://github.com/AmadoMuerte/Waxlight-launcher/pull/87)
- Stage 4 status: complete; favorite-server persistence, validation, and
  public-catalog browsing extracted from `application.Service`
- Stage 5 branch: `refactor/backend-mods-moddb`
- Stage 5 pull request: [#88](https://github.com/AmadoMuerte/Waxlight-launcher/pull/88)
- Stage 5 status: complete; installed-mod orchestration, the downloaded-mod
  cache, ModDB browsing, dependency resolution, and ModDB task tracking
  extracted from `application.Service`
- Stage 6 branch: `refactor/backend-snapshots-recovery`
- Stage 6 pull request: [#89](https://github.com/AmadoMuerte/Waxlight-launcher/pull/89)
- Stage 6 status: complete; snapshot, recovery, and Last Known Good ownership
  extracted from `application.Service`
- Stage 7 branch: `refactor/backend-updates-telemetry-statistics`
- Stage 7 pull request: [#90](https://github.com/AmadoMuerte/Waxlight-launcher/pull/90)
- Stage 7 status: merged into `dev` after successful local validation and CI
- Stage 8 branch: `refactor/backend-wails-transport`
- Stage 8 status: in progress
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

### Completed Stage 3 Slice: Package Import/Export and Mutation Locking

- Added an instance-owned `PackageService` with focused repository, creator,
  version, mod-store, catalog, downloaded-mod, catalog-installer, identity,
  archive-IO, mutation-gate, publisher, directory-removal, clock, and
  ID-generation boundaries.
- Moved package export, inspection, and import orchestration out of
  `application.Service` without changing Wails methods, DTOs, or messages.
- Added `mutations.Slot` as the neutral per-instance busy marker shared by
  instance mutations, snapshots, cloning, and launches; releases only clear a
  matching marker so stale cleanups can never free a newer operation.
- Added `internal/launching.Registry` owning the running-game map and the
  launch serialization; `Guard` (running check + slot reservation) and `Lock`
  (slot only) now back instance delete/clone guards, snapshot reservations,
  and mod mutations.
- Moved game-version-change preparation into `instances.UpdateService` with a
  narrow `SafetySnapshotter` port implemented by the application layer until
  the snapshot feature owns it.
- Moved package models handling behind `PackageIO`/`PackageArchive` ports with
  an archive adapter in `infrastructure/instancepackage`.

### Completed Stage 3 Slice: Launching and Processes

- Added `internal/launching` as the owner of launch validation, start, stop,
  running-game tracking, credential injection and cleanup, launcher-owned
  diagnostic logs, play-session persistence, and failed-startup handling.
- Moved process execution and OS-specific stop/kill behavior under
  `internal/platform/process` with `Running`/`Launcher` contracts.
- Moved the game-output error tailer into `internal/launching`.
- Moved startup reconciliation, account-cleanup, and the data-folder busy
  check into the launch coordinator.
- Made launch establishment recording lifecycle-owned and shutdown-aware;
  Last Known Good recording stays behind the narrow `LaunchRecovery` port.
- Removed the launch family, running state, guards, and package orchestration
  from `application.Service`; the instance services now receive the registry
  and slot at construction.

### Instance Feature

- [x] Add `internal/instances` as the owner of the instance model, inputs,
  repository interfaces, statuses, and instance-specific errors.
- [x] Split instance behavior into focused capabilities for:
  - queries and CRUD (complete);
  - directory allocation and local storage (creation and deletion allocation
    extracted; restore storage remains with the snapshot feature);
  - cloning (complete);
  - package import and export (complete);
  - update analysis and game-version changes (complete).
- [x] Replace the broad application store dependency with narrow instance,
  version, account, mod, snapshot, and filesystem ports.
- [x] Move instance mutation locking into the feature and integrate it with
  `internal/mutations.Gate`.
- [x] Move instance SQLite mappings to instance-owned models while preserving
  the current schema and stored rows.
- [x] Remove migrated instance methods, state, and tests from
  `internal/application.Service`.

### Launching and Processes

- [x] Add `internal/launching` with a focused launch coordinator that separates:
  - instance and version resolution;
  - account and session validation;
  - client-settings preparation and cleanup;
  - process argument and environment construction;
  - process start, stop, tracking, and exit handling;
  - launcher-owned diagnostic logs;
  - play-session persistence;
  - failed-startup and crash handling.
- [x] Move process execution and OS-specific stop/kill behavior under
  `internal/platform/process`.
- [x] Make every launch worker lifecycle-owned and joined during shutdown.
- [x] Preserve temporary credential injection into `clientsettings.json` and
  guaranteed cleanup after normal exit, failed launch, forced stop, and startup
  reconciliation.
- [x] Keep credentials out of process arguments, environment variables, logs,
  DTOs, errors, and events.
- [x] Preserve current launch validation messages, server-connect behavior,
  game events, and frontend controller contracts.

### Sessions and Statistics Inputs

- [x] Add `internal/sessions` as the owner of play-session models and repository
  interfaces.
- [x] Move session start/finish recording and interrupted-session recovery out
  of `application.Service`.
- [x] Expose narrow read capabilities that a later statistics feature can use.
- [x] Preserve playtime, crash, exit-code, process-ID, and account associations.

### Stage 3 Validation and Delivery

- [x] Add direct tests for instance CRUD, clone, package operations, validation,
  launch rollback, credential cleanup, process failures, shutdown, and session
  recovery.
- [x] Run focused race tests for instances, launching, sessions, bootstrap,
  SQLite, and presentation.
- [x] Confirm all instance/game Wails methods, DTOs, events, and error messages
  remain compatible.
- [x] Run the complete local validation matrix.
- [x] Complete manual smoke testing for create/edit/clone/import/export,
  authenticated and offline launch, stop, crash, and restart recovery.
- [x] Commit, synchronize with `origin/dev`, push, open a pull request against
  `dev`, pass CI, and merge before Stage 4 begins.

## Stage 4: Servers and Public Catalog

Goal: isolate favorite-server persistence and public-server browsing while
keeping Vintage Story protocol behavior in `vintagestory-go`.

### Completed Stage 4 Slice: Server Feature

- Added `internal/servers` as the owner of favorite/public server models,
  `SaveInput`, validation, repository ports, and the public-catalog port.
- Removed `domain.FavoriteServer` and `domain.PublicServer` without
  compatibility aliases.
- Split favorite-server CRUD (`Service`) from public-catalog queries
  (`CatalogService`); the catalog dependency is immutable at construction.
- Kept public catalog access behind the existing thin `vintagestory-go`
  adapter in `internal/infrastructure/servercatalog`, which now maps into
  `servers.PublicServer` and still exposes `NewClient`/`NewClientWithURL`
  for tests.
- Made the broad application store compose `servers.Repository` instead of
  redeclaring its methods, following the `instances.Repository` pattern.
- Removed `ConfigurePublicServerCatalog`, the `serverCatalog` field, and the
  favorite-server methods, input, and `PublicServerCatalog` port from
  `application.Service` and `application.Store`.
- Moved the `ServerController` with its `SaveFavoriteServerRequest`,
  `FavoriteServerDTO`, and `PublicServerDTO` into `internal/transport/wails`
  while preserving controller, method, argument, and JSON field names.
- Regenerated Wails bindings; the server binding moved from the
  `presentation` namespace to the `wails` namespace, and the frontend bridge
  now resolves both namespaces so controllers can migrate incrementally.
- Preserved favorite-server rows, name/address validation, instance
  association, creation-timestamp preservation, the
  `favorite-server:updated`/`favorite-server:removed` events, mutation-gate
  coverage, public catalog fields, and the whitelist-listing-without-address
  behavior.
- Kept `ServerLaunchRequest`, connect-on-launch validation, and
  `--connect` argument construction with the `launching.Coordinator`
  extracted in Stage 3.
- Added direct feature tests for favorite CRUD, trimming, validation,
  instance association, created-at preservation, mutation gating, event
  payloads, and repository/catalog failure propagation, plus controller and
  DTO round-trip tests in `internal/transport/wails`.

### Stage 4 Validation

The complete local validation matrix passed on 2026-08-11:

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

### Stage 4 Checkboxes

### Server Feature

- [x] Add `internal/servers` as the owner of favorite/public server models,
  validation, repository ports, and launch requests. (Launch requests and
  connect-on-launch behavior stay with the Stage 3 `launching` coordinator.)
- [x] Split favorite-server CRUD from public-catalog queries.
- [x] Keep public catalog access behind a thin `vintagestory-go` adapter.
- [x] Remove `ConfigurePublicServerCatalog` and inject immutable dependencies at
  construction.
- [x] Move the server controller into `internal/transport/wails` while
  preserving controller and method names.
- [x] Preserve favorite-server rows, address validation, instance association,
  public catalog fields, and connect-on-launch behavior.

### Stage 4 Validation and Delivery

- [x] Add tests for favorite-server CRUD, validation, catalog mapping, catalog
  failures, and server launches.
- [x] Confirm server DTOs, generated bindings, events, and frontend calls remain
  compatible.
- [x] Run focused race tests and the complete local validation matrix.
- [ ] Complete manual smoke testing for favorites, public browsing, refresh,
  errors, and joining a server (pending before merge).
- [ ] Commit, synchronize, push, open a pull request against `dev`, pass CI, and
  merge before Stage 5 begins.

## Stage 5: Mods and ModDB

Goal: separate installed-mod orchestration from ModDB browsing and eliminate
the remaining mod state in `application.Service`.

### Upstream Vintage Story Work

- Inventoried the local `modinfo.json` parsing, dependency, compatibility, and
  modpack behavior against the released `vintagestory-go` API. The library
  already owned modpack analysis (`modpack.Analyze`) but had no modinfo
  parsing; Waxlight duplicated BOM-tolerant, lenient JSON parsing in two
  places.
- Contributed the generic `modinfo` package to `vintagestory-go`
  (`Parse` with byte-order-mark and trailing-comma tolerance plus
  `ReadArchive` for ZIP-based mod archives, size-capped) with upstream tests,
  released as `v0.2.0` (sentinel error refinement in `v0.2.1`), and consumed
  the new version in Waxlight.
- Removed the duplicated local modinfo implementations from
  `internal/infrastructure/filesystem` and `internal/application`.
- Kept only Waxlight-specific mapping, persistence, filesystem policy,
  orchestration, and user-facing errors in this repository.

### Installed Mods

- Added `internal/mods` as the owner of installed/downloaded mod models,
  repository ports, mutation coordination, feature errors, and identity
  helpers.
- Removed `domain.InstalledMod`, `domain.DiscoveredMod`, and every
  `domain/modcatalog.go` model without compatibility aliases.
- Moved the mod error codes (`ErrModNotFound`, `ErrModVersionNotFound`,
  `ErrModCatalog`, `ErrModIncompatible`, `ErrModAlreadyActive`,
  `ErrInvalidModFile`, `ErrModFileExists`) into the feature.
- Moved the mod telemetry constants into the feature; `internal/telemetry`
  references them so the allowlist can never drift from the emitted events.
- Added `internal/mods.Service` for installed-mod discovery and
  reconciliation, local file installation, enable/disable, removal with
  dependency previews, and binding local files to their catalog entries.
- Added `internal/mods.CatalogService` for ModDB browsing, catalog downloads
  with recursive dependency resolution, the downloaded-mod cache and cleanup,
  local-mod linking and upload, and update analysis and application.
- Added `internal/mods.ModTaskManager` as the dedicated ModDB task manager
  separate from the persistent operations subsystem: per-release
  reservations, cancellation, and the `mods:task-progress` and
  `mods:downloads-changed` events.
- Kept the mods feature independent of the instances feature through a minimal
  `InstanceRef` view; bootstrap maps the shared store to the feature's
  repository port.
- Kept safety snapshots behind the narrow `SafetySnapshotter` port until the
  snapshot feature owns them in Stage 6.
- Removed migrated mod methods, mutexes, task maps, and `ConfigureMods` from
  `application.Service`; the mods services are constructed inside
  `application.NewService` with immutable dependencies and exposed through
  `Service.Mods()` and `Service.ModsCatalog()`.
- Removed the transitional package-import adapters and `ModIdentity` delegate;
  the instance package service now receives the mods services and
  `mods.Identity` directly.
- Preserved installed-mod rows, reconciliation, install/update/removal
  behavior, dependency installation and previews, library cleanup,
  link/upload matching, update analysis through `vintagestory-go/modpack`, and
  every mod event name and payload.

### ModDB Browsing and Tasks

- Kept ModDB browsing behind the existing thin `vintagestory-go` adapter in
  `internal/infrastructure/modcatalog`, which now maps into `mods` models.
- Moved ModDB task tracking, cancellation, progress, and completion into
  `ModTaskManager`, separate from persistent launcher operations; the
  namespace test proves the two cancellation domains cannot cross.
- Removed `ConfigureMods`; the catalog, downloaded-store, downloader, and
  version dependencies are immutable at construction.
- Preserved ModDB query behavior, pagination, dependency flow, event names,
  DTOs, and frontend task cancellation.
- Moved the shared `modinfo.json` reading for downloaded archives and
  dependency resolution to the released `vintagestory-go/modinfo` package.

### Stage 5 Validation

The complete local validation matrix passed on 2026-08-11:

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

### Stage 5 Checkboxes

### Upstream Vintage Story Work

- [x] Inventory remaining local `modinfo.json`, dependency, compatibility, and
  modpack behavior against the released `vintagestory-go` API.
- [x] Contribute missing generic parsing or compatibility capabilities to
  `vintagestory-go` with upstream tests.
- [x] Release and consume the required library version before deleting any
  temporary local generic implementation.
- [x] Keep only Waxlight-specific mapping, persistence, filesystem policy,
  orchestration, and user-facing errors in this repository.

### Installed Mods

- [x] Add `internal/mods` as the owner of installed/downloaded mod models,
  repository ports, mutation coordination, and feature errors.
- [x] Split capabilities for:
  - installed-mod discovery and reconciliation;
  - local file installation and linking;
  - enable/disable and removal;
  - dependency installation and removal previews;
  - downloaded-mod cache and cleanup;
  - update analysis and application.
- [x] Define narrow filesystem, downloader, catalog, snapshot, instance, and
  event boundaries.
- [x] Preserve safety snapshots before destructive mod changes.
- [x] Remove migrated mod methods, mutexes, task maps, and state from
  `application.Service`.

### ModDB Browsing and Tasks

- [x] Add a focused ModDB catalog capability backed by `vintagestory-go`.
- [x] Move ModDB task tracking, cancellation, progress, and completion into a
  dedicated manager separate from persistent launcher operations.
- [x] Remove `ConfigureMods` and use immutable constructor dependencies.
- [x] Preserve current ModDB query behavior, pagination, dependency flow,
  event names, DTOs, and frontend task cancellation.

### Stage 5 Validation and Delivery

- [x] Add tests for discovery, reconciliation, local linking, dependencies,
  updates, cancellation, cache cleanup, unsafe archives, and rollback.
- [x] Run focused race tests for mods, ModDB tasks, downloader, snapshots,
  SQLite, bootstrap, and presentation.
- [x] Confirm all mod Wails methods, DTOs, events, and generated bindings remain
  compatible.
- [x] Run the complete local validation matrix.
- [x] Complete manual mod-management smoke tests (pending before merge).
- [x] Commit, synchronize, push, open a pull request against `dev`, pass CI, and
  merge before Stage 6 begins.

## Stage 6: Snapshots and Recovery

Goal: isolate backup, restore, Last Known Good, and crash-recovery policy behind
narrow repositories and filesystem boundaries.

### Snapshot Feature

- Added `internal/snapshots` as the owner of snapshot models (`Type`,
  `Reason`, `ModSource`, `Mod`, `Manifest`, `InstanceSnapshot`), format
  constants, feature error codes, operation title keys, retention policy, and
  capabilities.
- Removed `domain.SnapshotType`, `domain.SnapshotReason`, `domain.SnapshotMod`,
  `domain.SnapshotManifest`, and `domain.InstanceSnapshot` without
  compatibility aliases.
- Moved manual snapshot creation, automatic safety snapshots, restore
  coordination (format v1 and v2), automatic retention, exact managed-mod
  restoration, and credential sanitization out of `application.Service`.
- Defined narrow snapshot ports for storage, instance, game-version, mod-store,
  catalog, archive-info, settings, mutation-gate, per-instance lock/slot,
  disk-space, client-settings sanitize/clear, log hardening, total-size
  enumeration, safe directory removal, and Last-Known-Good reference.
- Made `internal/snapshots` independent of the mods, instances, launching, and
  recovery features through minimal views and structural ports; the source
  marker format (`moddb:<modID>:<versionID>`) is parsed and rendered in the
  snapshot feature to match the mods feature contract.
- Moved the snapshot storage adapter from `internal/infrastructure/snapshotstore`
  to `internal/platform/snapshots` while preserving the directory layout,
  manifest format, staging, listing, removal, and size semantics.
- Kept `clientsettings.json` sanitized on every snapshot copy and every restore
  staging copy so `sessionkey`, `sessionsignature`, `playeruid`, and
  `playername` never enter snapshot, export, or archive paths.
- Preserved restore staging, atomic directory swap with rollback, marker
  rewriting, log hardening, bounded parallel mod downloads, checksum and
  identity validation, exact record rebuild, operation progress events, and
  user-facing snapshot errors and messages.
- Kept the LKG reference out of the snapshot feature through a
  `LastKnownGoodReference` port implemented by the recovery feature: deletion
  clears the marker reference and retention protects the active recovery
  snapshot.
- Removed the snapshot methods, `snapshotstore.Store` field, `snapshots.go`,
  `safetysnapshots.go`, snapshot operation titles, and `titleParams` from
  `application.Service`; the service now exposes `Snapshots()`.

### Recovery Feature

- Added `internal/recovery` as the owner of Last Known Good state
  (`LastKnownGood`), `ModChange`, `ConfigurationChanges`,
  `LastKnownGoodStatus`, and `RecoverySuggestion` models, the repository port,
  failed-launch analysis, recovery suggestions, and restore coordination.
- Removed `domain.LastKnownGood`, `domain.ModChange`,
  `domain.ConfigurationChanges`, `domain.LastKnownGoodStatus`, and
  `domain.RecoverySuggestion` without compatibility aliases; SQLite mappings
  now use the recovery-owned model with the identical JSON representation.
- Moved `RecordLastKnownGood`, failed-launch comparison, recovery-snapshot
  resolution, reference clearing, and status reads out of
  `application.Service`.
- Kept the current crash-window behavior: the launching coordinator's
  `LaunchRecovery` port is implemented structurally by the recovery service
  with the same `last-known-good:updated` and `game:recovery-suggestion`
  events and the same state-signature suppression contract.
- Reads snapshot capabilities through narrow `SnapshotReader` and
  `ModConfiguration` ports implemented by the snapshots feature; the snapshot
  `ModKey`/`SameModSet`/`ValidateMods`/`ModDisplayName` helpers are reused
  instead of duplicated.
- Moved the configuration comparison, mod-key, and state-signature unit tests
  into the recovery feature package.
- Removed the recovery methods and `lastknown.go` from `application.Service`;
  the service now exposes `Recovery()`.

### Wiring and Dependency Direction

- `mods` and `instances` now consume `snapshots.SafetySnapshotter` directly
  with the feature-owned `snapshots.Reason` type and reason constants; their
  duplicate snapshotter ports and func adapters were deleted.
- `launching` references the snapshot feature error codes
  (`snapshots.ErrSnapshotInProgress`) instead of the removed domain codes.
- `internal/snapshots` depends only on domain errors, operations, settings,
  versions, and structural ports; it does not import mods, instances,
  launching, recovery, or platform adapters.
- `application.NewService` now receives the snapshot storage adapter, total-size
  enumeration, client-settings sanitization, and log hardening as immutable
  parameters; direct application imports of the snapshot, data-root, and
  filesystem adapters were removed.
- Removed the snapshot and recovery error codes from `internal/domain`.

### Stage 6 Validation

- Added direct integration tests for the snapshot storage adapter under
  `internal/platform/snapshots`: manifest round trips, newest-first listing,
  corrupted-directory skipping, name-mismatch rejection, unsafe identifiers,
  removal, size, and staging-directory isolation.
- Added direct snapshot feature tests: credential sanitization, managed-release
  manifest recording, disabled safety snapshots, automatic retention with LKG
  protection, delete reference clearing, un-restorable mod rejection, failed
  restores, restorability validation, and manifest JSON contract.
- Added direct recovery feature tests: marker persistence and events, snapshot
  linking, failed-launch suggestions, recovery-snapshot fallback, reference
  clearing, protected-snapshot resolution, and status reads.
- Kept the end-to-end application tests for manual/safety snapshots, retention,
  exact restore, credential sanitization, failed restores, Last Known Good, and
  crash recovery through the new feature services.
- Passed `go test ./...`, `go test -race ./...`, `go vet ./...`,
  `make format-check`, `make lint`, `make security`, all frontend tests, the
  frontend production build, `make wails-build` on Linux AMD64, and
  `git diff --check`.

### Stage 6 Checkboxes

### Snapshot Feature

- [x] Add `internal/snapshots` as the owner of snapshot models, reasons,
  retention policy, repository ports, and capabilities.
- [x] Move manual snapshots, safety snapshots, restore coordination, pruning,
  and exact managed-mod restoration out of `application.Service`.
- [x] Define narrow snapshot filesystem, instance, mod, version, and mutation
  ports.
- [x] Move the snapshot storage adapter under `internal/platform/snapshots`.
- [x] Ensure every snapshot, export, and archive path removes `sessionkey`,
  `sessionsignature`, `playeruid`, and `playername` from `clientsettings.json`.

### Recovery Feature

- [x] Add `internal/recovery` for Last Known Good state, startup
  reconciliation, failed-launch analysis, recovery suggestions, and restore
  coordination.
- [x] Preserve current crash-window behavior and user-facing recovery events.
- [x] Remove direct application imports of snapshot, data-root, and filesystem
  adapters.
- [x] Remove migrated snapshot/recovery methods and state from
  `application.Service`.

### Stage 6 Validation and Delivery

- [x] Add direct integration tests for the snapshot storage adapter.
- [x] Add tests for manual/safety snapshots, retention, exact restore,
  credential sanitization, failed restores, Last Known Good, and crash
  recovery.
- [x] Run focused race tests and the complete local validation matrix.
- [x] Complete manual smoke testing for create, restore, prune, failed launch,
  and suggested recovery.
- [x] Commit, synchronize, push, open a pull request against `dev`, pass CI, and
  merge before Stage 7 begins.

## Stage 7: Updates, Telemetry, Statistics, and Game Logs

Goal: give the remaining operational services explicit feature ownership and
lifecycle-safe background delivery.

### Completed Stage 7: Launcher Updates

- Added `internal/updates` as the owner of the launcher update model (`Update`,
  `Progress`), the update lifecycle stages, update error codes, the source,
  installer, signature-verifier, mutation-gate, and telemetry ports, and the
  update service.
- Removed `domain.LauncherUpdate` and `domain.LauncherUpdateProgress` without
  compatibility aliases.
- Moved the update error codes used by the updater into the feature
  (`ErrUpdateUnavailable`, `ErrUpdateInProgress`, `ErrUpdateFailed`,
  `ErrUpdateUnsupported`, `ErrUpdateDownloadFailed`,
  `ErrUpdateSignatureInvalid`, `ErrUpdateInstallerStartFail`) while keeping
  their exact string values; unused legacy update codes remain in
  `internal/domain`.
- Replaced `application.LauncherUpdateService` and its
  `ConfigureTelemetry` method with `updates.Service`, which receives source,
  downloader, installer, signature verifier, mutation gate, data root, current
  version, and telemetry as immutable constructor dependencies.
- Moved `PurgeStaleUpdateSessions` and `normalizeUpdateChannel` into the
  feature.
- Kept the `updates:progress` event contract: the published phase strings
  (`checking`, `downloading`, `signature`, `installing`, `restarting`) match
  the frontend `LauncherUpdateProgress` union exactly.
- Kept trusted URL validation, SHA-256 manifest checksums, signature
  verification, publisher trust on Windows, channel behavior, portable-mode
  guards, installer guarantees, and the update telemetry events
  (`update_started`, `update_failed` with the download/signature/install error
  taxonomy).
- Made the GitHub source adapter map into `updates.Update` instead of
  `domain.LauncherUpdate`.
- Moved the updater tests into the feature and added direct telemetry tests
  for the update started/failed boundaries.
- Kept the `LauncherUpdateController` Wails methods, arguments, DTO fields,
  and events unchanged.

### Completed Stage 7: Telemetry

- Made every telemetry delivery worker lifecycle-owned through a new
  `telemetry.WorkerGroup` port (`Go(func(context.Context)) bool`) implemented
  by `*app.Lifecycle` at the composition root: `Event`, `Error`, and
  `MaybeSendHeartbeat` schedule their delivery through the worker group, the
  worker context cancels in-flight sends during shutdown, and shutdown joins or
  safely abandons the delivery workers.
- Made a refused heartbeat worker reset the pending flag so a shutdown race can
  never block future heartbeats.
- Preserved disabled-until-opt-in behavior, event/error allowlists, identity
  privacy, consent synchronization through `SynchronizeConsent`, and the
  heartbeat policy.
- Removed the remaining post-construction telemetry configuration:
  `application.Service.ConfigureTelemetry` and
  `launching.Coordinator.SetTelemetry` are gone; the application service and
  the launch coordinator receive their telemetry port at construction.
- Wired the application instance-deletion telemetry through the already
  injected `mods.Telemetry` port instead of a service-held telemetry field.
- Kept the telemetry package framework-free (no Wails or app imports).

### Completed Stage 7: Statistics

- Added `internal/statistics` as the owner of the aggregated statistics model
  (`Statistics`), the recent-session limit, and the `Overview` and
  `InstancePlaytime` capabilities.
- Exposed the narrow `sessions.Reader` read capability
  (`SessionStatistics`, `ListSessions`, `InstancePlaytime`) from
  `sessions.Service`; `sessions` no longer owns statistics aggregation.
- Removed `sessions.Statistics`, `sessions.Service.GetStatistics`, and
  `sessions.Service.GetInstancePlaytime` without compatibility aliases; the
  repository-level `sessions.StatisticsTotals` stays with the sessions feature.
- Moved the statistics aggregation tests into the statistics feature and kept
  the end-to-end backend statistics test.
- Kept the `StatisticsController.GetOverviewStatistics` Wails method, the
  `StatisticsDTO` JSON fields, and the per-instance playtime used by the
  instance list unchanged.

### Completed Stage 7: Game Logs

- Added `internal/gamelog` as the owner of game-log tailing, crash-indicator
  classification, and launcher-side crash-report coordination; the launching
  coordinator now calls `gamelog.Watch(instance.Name, logPath)`.
- Moved the game-output tailer, the error-line classifier, the rate limiter,
  the truncation handling, and the `crashreport-*.txt` coordination out of
  `internal/launching` without changing any behavior.
- Preserved the log console flow: forwarded error lines and crash reports
  reach the console, session log files, and support exports through the shared
  slog pipeline, and full game output is never copied into launcher logs.
- Moved the tailer tests into the feature unchanged.

### Launcher Updates

- [x] Add `internal/updates` for update checks, download orchestration,
  verification, installation, skip policy, and update events.
- [x] Make telemetry, downloader, platform, publisher, and installer
  dependencies immutable.
- [x] Remove `LauncherUpdateService.ConfigureTelemetry`.
- [x] Preserve trusted URL validation, checksums, signatures, publisher trust,
  channel behavior, skipped versions, and installer guarantees.

### Telemetry

- [x] Move telemetry orchestration and background delivery into the final
  feature/platform structure.
- [x] Keep telemetry disabled until explicit opt-in and preserve event/error
  allowlists, identity privacy, consent synchronization, and heartbeat policy.
- [x] Make every telemetry worker lifecycle-owned, cancellable where possible,
  and joined or safely abandoned during shutdown.
- [x] Remove remaining post-construction telemetry configuration.

### Statistics and Game Logs

- [x] Add `internal/statistics` for launcher statistics and per-instance
  playtime queries over the sessions read capability.
- [x] Add `internal/gamelog` for game-log tailing, crash indicators, and
  launcher-side crash-report coordination.
- [x] Preserve lazy frontend loading of the log console and existing events.
- [x] Never copy game output into launcher logs or support exports.

### Stage 7 Validation and Delivery

- [x] Add tests for update trust failures, cancellation, telemetry opt-in and
  allowlists, shutdown, statistics, log tailing, and crash coordination.
- [x] Run focused race, privacy, security, and complete local validation checks.
- [x] Complete manual smoke testing for update UI, telemetry transitions,
  statistics, and game-log viewing.
- [x] Commit, synchronize, push, open a pull request against `dev`, pass CI, and
  merge before Stage 8 begins.

## Stage 8: Wails Transport and Composition Root

Goal: reduce Wails to a transport adapter and make `internal/app` the complete,
explicit composition root.

### Wails Transport

- Moved every remaining controller and DTO from `internal/presentation` into
  feature-oriented files under `internal/transport/wails`:
  - `app_controller.go`, `account_controller.go`, `game_version_controller.go`,
    `instance_controller.go`, `mod_manager_controller.go`,
    `mod_catalog_controller.go`, `instance_package_controller.go`,
    `launch_controller.go`, `statistics_controller.go`,
    `operation_controller.go`, `snapshot_controller.go`,
    `last_known_good_controller.go`, `log_controller.go`,
    `settings_controller.go`, `update_controller.go` (the server controller
    already lived in the transport package).
- Moved the DTO layer into `dto.go` (shared DTOs and conversions),
  `modcatalog_dto.go`, `instance_update_dto.go`, `instancepackage_dto.go`,
  `snapshot_dto.go`, and `lastknown_dto.go`.
- Deleted the entire `internal/presentation` package; its tests moved with the
  controllers (DTO serialization, support-log formatting, URL validation, and
  the credential allow-list security tests).
- Kept Wails limited to parameter/DTO conversion, feature invocation, dialogs,
  events, browser opening, directory opening, and application quit.
- Removed the last `application.Service` references from controllers: the
  instance controller dropped its unused service field, and the log controller
  now consumes the focused instance query service through a narrow
  `instanceLister` port for its support-log summary.
- Preserved every frontend-consumed controller name, method name, argument
  order, return shape, JSON field, event, and user-facing error; regenerated
  bindings contain only the intentional namespace change (`presentation` ->
  `wails`) with identical controller methods and model classes.
- Made the transport consume a minimal local `lifecycle` interface
  (`Context()` and `Go`) instead of `*app.Lifecycle`; the composition root can
  therefore bind the controllers without creating an import cycle.
- Updated the frontend bridge to resolve only the `wails` namespace and moved
  `accounts.ts` onto the `wailsjs/go/wails` bindings.

### Wails API Inventory and Compatibility Check

- Added the checked-in `docs/wails-api-inventory.json` generated from the
  transport controllers by `go run ./internal/transport/wails/inventory`
  (`make api-inventory`).
- Added automated compatibility tests in `internal/transport/wails`:
  - the inventory always matches the transport controller methods;
  - the generated `frontend/src/wailsjs/go/wails` bindings expose exactly the
    inventoried controllers and methods;
  - every `call(...)` and binding import in `frontend/src/shared/api` targets a
    controller and method that exist in the inventory, no legacy `presentation`
    namespace references remain, and `models.ts` still exports the `wails`
    namespace.

### Composition Root

- Moved dependency construction and wiring into `internal/app/wire.go`: the
  `Container` type, `New`/`NewWithHome` (an explicit home directory keeps the
  composition root testable), `Startup`, `Shutdown`, and every construction
  adapter (`newVersionID`, `credentialStoreUnavailable`, package adapters).
- Lifecycle, mutation gate, event publisher, repositories, features, platform
  adapters, and transport assembly are explicit and immutable at construction;
  no post-construction wiring remains except the launch account-cleanup hook
  that already existed.
- `cmd/waxlight/main.go` is now a small executable entrypoint that constructs
  the container and starts Wails.
- Deleted `internal/bootstrap`; installer telemetry consent moved into
  `internal/app` unchanged.
- Added `internal/apptest.Lifecycle` so feature tests that cannot import the
  composition root still exercise lifecycle-owned workers.

### Composition Tests

- `internal/app/wire_test.go` proves:
  - the bound controllers exactly match the checked-in Wails API inventory;
  - recovery ordering: interrupted play sessions and queued/running operations
    seeded before construction are recovered before the container is returned;
  - startup ordering: the lifecycle context derives from the framework context
    and accepts lifecycle-owned workers;
  - deterministic shutdown: cancelling joins a blocked lifecycle worker and
    leaves the store consistent for the next run.

### Stage 8 Checkboxes

### Wails Transport

- [x] Move every remaining controller and DTO from `internal/presentation` into
  feature-oriented files under `internal/transport/wails`.
- [x] Keep Wails limited to parameter/DTO conversion, feature invocation,
  dialogs, events, browser opening, directory opening, and application quit.
- [x] Preserve all frontend-consumed controller names, method names, argument
  order, return shapes, JSON fields, events, and user-facing errors.
- [x] Regenerate bindings and verify that only intentional package-path changes
  occur.
- [x] Add a checked-in Wails API inventory and an automated frontend-to-backend
  compatibility check.

### Composition Root

- [x] Move dependency construction and wiring into `internal/app/wire.go`.
- [x] Keep lifecycle, mutation gate, event publisher, repositories, features,
  platform adapters, and transport assembly explicit and immutable.
- [x] Keep `cmd/waxlight/main.go` as a small executable entrypoint.
- [x] Remove `internal/bootstrap` after all startup, reconciliation, and shutdown
  responsibilities have moved.
- [x] Add composition tests that prove startup ordering, recovery ordering, and
  deterministic shutdown.

### Stage 8 Validation and Delivery

- [ ] Verify the complete Wails API inventory against frontend consumers and
  generated bindings.
- [ ] Run focused lifecycle, bootstrap/composition, transport, and race tests.
- [ ] Run the complete local validation matrix and a desktop smoke test.
- [ ] Commit, synchronize, push, open a pull request against `dev`, pass CI, and
  merge before Stage 9 begins.

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
