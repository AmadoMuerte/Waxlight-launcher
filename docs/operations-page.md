# Operations page contract

This document is the implementation contract for Waxlight's **Operations**
page. Read it before changing operation persistence, cancellation, progress
events, or `frontend/src/pages/OperationsPage.tsx`.

The MVP interface is English-only. Keep user-facing text on this page in
English until the planned RU/EN localization layer is introduced; do not mix
languages directly in React components.

## Purpose and statuses

The page shows persisted long-running work such as game-version downloads and
installations. An operation has one of the centralized frontend statuses:

- `queued` and `running` are active;
- `completed`, `failed`, and legacy `cancelled` records are finished.

New explicit cancellations are not history entries. A successful cancel is a
rollback: Waxlight removes the download artifacts, removes the SQLite row, emits
`operation:removed`, releases the per-version lock, and only then resolves the
cancel call. A `cancelled` row may still exist in databases created by older
versions and can be removed with either history action.

## Layer ownership

The page follows the project layering rules:

- `frontend/src/pages/OperationsPage.tsx` renders rows and owns only local UI
  state, confirmation prompts, and notifications;
- `frontend/src/shared/api/index.ts` is the only frontend transport wrapper;
- `internal/presentation/controllers.go` exposes thin Wails methods;
- `internal/application/service.go` and `versions.go` own use-case behavior;
- `internal/infrastructure/database/sqlite.go` owns persistence and enforces the
  finished-only deletion guard;
- the shared downloader still owns HTTP transfer, checksum verification,
  progress, and resumable `.partial` files.

Do not call generated Wails bindings directly from the page and do not move
cleanup or status rules into React.

## Public operations API

The current Wails-facing contract is:

```text
ListOperations() -> OperationDTO[]
CancelOperation(id) -> void
DeleteOperation(id) -> void
ClearOperationHistory() -> number
```

`DeleteOperation` accepts only finished operations. `ClearOperationHistory`
deletes every finished row and returns the number removed. Both protections are
implemented in SQLite with terminal-status predicates, so a stale or malicious
frontend cannot delete active work.

The list is currently ordered newest-first and limited to 100 records.

## Cancellation cleanup

Official game packages are downloaded to:

```text
<data-root>/downloads/<catalog-filename>.partial
```

The shared downloader renames that file to the same path without `.partial`
after verification. On a network or application error the partial file is kept
for a later resume. On an explicit user cancel, the application service instead:

1. cancels the operation context;
2. waits for the downloader or installer to stop;
3. removes both `<filename>.partial` and `<filename>` when present;
4. transitions the row internally to a terminal state and deletes it through
   the guarded store method without publishing a transient cancelled row;
5. releases operation/version locks and emits `operation:removed`;
6. resolves `CancelOperation`, after which the page refreshes.

If filesystem cleanup fails, the operation is marked `failed` and the cancel
call returns an error. Keeping that failed row is intentional: it exposes the
cleanup problem and lets the user retry manual history deletion after fixing
permissions.

## Page behavior

- Active rows have **Cancel**. The action is disabled while another page action
  is pending.
- Finished rows have **Delete** and require confirmation.
- **Clear history** is shown only when at least one finished row is present and
  requires confirmation. Its copy explicitly says active operations are kept.
- After every successful mutation the page refreshes from the backend; it does
  not optimistically invent operation state.
- Progress and speed remain read-only projections of backend values.

The backend publishes Wails operation events. The current frontend refreshes
the collection every eight seconds and immediately after page actions; it does
not yet subscribe to those events. A future live subscription should consume
the existing shared event names instead of adding page-specific event
infrastructure.

## Required regression coverage

Keep tests for these invariants:

- cancel deletes the real `.partial` file and the database row;
- the same version can start again after cancellation;
- single deletion rejects active rows;
- bulk clearing removes `completed`, `failed`, and legacy `cancelled` rows but
  retains `queued`/`running` rows;
- the page exposes Cancel and Delete only for their matching status groups;
- delete and clear call the shared API wrapper, refresh, and notify.

Do not add retry or pause controls until the corresponding application use case
is implemented. A visible button without backend behavior violates this page's
contract.
