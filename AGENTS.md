# Waxlight Launcher

## Workflow

- Frontend lives in `frontend/`; install its locked dependencies with `npm ci --include=dev --prefix frontend`. This also registers the Husky pre-commit hook from `frontend/.husky`.
- When implementation depends on an external library API, use Context7 to verify the current documentation before coding.
- `make test` first runs the frontend production build (including i18n and TypeScript checks), then Go and frontend tests.
- Run focused Go tests with `go test ./path/to/package -run TestName`.
- Run `make format` to format Go and frontend sources. Use `make format-check` to validate `gofmt` and `oxfmt --check` without changing files.
- Run `make lint` for Linux-targeted Go static analysis and frontend `oxlint`; run `make vet` for Go static analysis on the current platform. Run `make security` for prohibited-pattern and vulnerability checks.
- Pre-commit runs `make format-check lint` and blocks commits on failure. Use `git commit --no-verify` only for an emergency bypass.
- Full release validation requires a version: `make release-check VERSION=X.Y.Z`.
- `make release VERSION=X.Y.Z` is a release-only command and must be run from a clean, synchronized `main` branch. It commits version files, pushes `main`, and creates and pushes a release tag.
- Never run release commands from `dev`, feature branches, fix branches, or pull request branches.
- Run `wails dev` only from `cmd/waxlight`. Build supported desktop artifacts with `make wails-build`; plain `go build` without Wails desktop tags is not a supported GUI build.
- Native credential-store integration tests are tagged and require Linux Secret Service or Windows Credential Manager; CI provides those platform services.

## Git Branching And Pull Requests

### Branch roles

- `main` is the stable production branch. It contains only code that has already passed CI and integration testing and is ready for release.
- `dev` is the integration and testing branch. Normal development work is merged into `dev` first.
- Never use `main` as the base branch for normal feature, fix, refactor, documentation, dependency, or maintenance work.
- Never commit or push directly to `main`.
- Never commit or push directly to `dev` unless the task explicitly requires an administrative branch-maintenance change that cannot reasonably go through a pull request.
- Never force-push `main` or `dev`.
- Never delete `main` or `dev`.
- Never create a release tag from `dev` or any working branch.

### Working branches

- Start normal work from the latest `dev`:
  ```bash
  git switch dev
  git pull --ff-only origin dev
  git switch -c <type>/<short-description>
  ```
- Use descriptive branch prefixes such as:
  - `feat/` for features
  - `fix/` for bug fixes
  - `refactor/` for refactors
  - `chore/` for maintenance
  - `docs/` for documentation
  - `test/` for test-only changes
  - `ci/` for CI changes
- Keep working branches focused on one logical change. Do not mix unrelated cleanup into the same pull request.

### Pull request targets

- All normal pull requests must target `dev`.
- When using GitHub CLI for normal development, explicitly set the base branch:
  ```bash
  gh pr create --base dev
  ```
- A pull request from a feature/fix/refactor/chore/docs/test/ci branch directly into `main` is forbidden.
- The only normal pull request allowed to target `main` is:
  ```text
  dev -> main
  ```
- `dev -> main` is the release/integration promotion pull request and must only be opened after the current `dev` state has been tested and is considered release-ready.
- Do not change the base branch from `dev` to `main` merely to make a pull request mergeable.
- If a task or agent session started from `main` by mistake and the work is not a release promotion, move the work onto a branch based on `dev` before opening the pull request.

### Required validation

- CI must run for pull requests targeting both `dev` and `main`.
- Do not merge a pull request while required checks are failing, cancelled, pending indefinitely, or missing.
- The protected branches are expected to require the repository's configured checks, including tests/static checks, security checks, native credential-store integration checks, and Linux/Windows production builds.
- Do not bypass branch protection, required checks, or pull-request requirements just to merge faster.
- Do not modify repository branch protection, rulesets, required checks, or merge policies unless the task explicitly asks for repository-administration changes.
- Resolve review conversations before merging when branch protection requires it.

### Promotion and release flow

Normal development:

```text
feat/* / fix/* / refactor/* / chore/* / docs/* / test/* / ci/*
                              |
                              v
                             dev
                              |
                    integration testing
                              |
                              v
                         dev -> main
                              |
                              v
                             main
                              |
                              v
                           release
```

Before promoting `dev` to `main`:

1. Ensure `dev` is up to date and CI is green.
2. Run the relevant local validation for the accumulated changes.
3. Test the launcher behavior that cannot be fully covered by automated CI.
4. Open a pull request from `dev` to `main`.
5. Merge only after all required checks pass.
6. Update local `main` with `git pull --ff-only origin main`.
7. Run release validation from `main`.
8. Run `make release VERSION=X.Y.Z` only when the release is intentionally being published.

## Structure

- `cmd/waxlight/main.go` is a small entrypoint: installs the logger, then
  calls `app.New()` and starts Wails. Keep feature logic independent of Wails
  and React.
- The backend is a feature-oriented modular monolith with hexagonal boundaries.
  See `docs/backend-architecture.md` for the full package map, dependency
  rules, lifecycle, events, migrations, security boundaries, and extension
  checklist.
- `internal/app/wire.go` is the complete, explicit composition root: it
  constructs every dependency once (features, platform adapters, controllers)
  and owns `Startup`/`Shutdown`. Store-to-feature mappings that belong to no
  feature live in `internal/app/adapters.go`. Never add a service constructor
  call outside `wire.go`.
- Features (`internal/accounts`, `instances`, `versions`, `launching`,
  `sessions`, `mods`, `snapshots`, `recovery`, `servers`, `settings`,
  `operations`, `updates`, `telemetry`, `statistics`, `gamelog`, `downloads`,
  `events`, `mutations`, `errs`, ...) own their models, ports, use cases, and
  tests. Features never import `internal/app`, `internal/transport/wails`, or
  `internal/platform/*`; they consume other features only through narrow ports
  and structural views.
- Adapters live in `internal/platform/*` (`sqlite`, `snapshots`, `process`,
  `logging`, `dataroot`, `credentials`, `filesystem`, `downloader`,
  `vintagestory`, ...) and are framework-free. Vintage Story access goes
  through the thin `platform/vintagestory`, `platform/modcatalog`, and
  `platform/servercatalog` adapters.
- Wails controllers and DTOs live in `internal/transport/wails` (feature
  files); they only convert parameters/DTOs, call features, emit events, and
  open dialogs/browser/directories. Bindings are generated under
  `frontend/src/wailsjs`; the checked-in API inventory is
  `docs/wails-api-inventory.json` (`make api-inventory`).
- User-facing errors use `internal/errs` (`errs.NewError`/`AppError`); the
  code strings are a frontend contract (i18n) and must never change.
- Runtime SQLite schema changes belong in
  `internal/platform/sqlite/migrations.go`; files in `migrations/` are not
  canonical runtime migrations.
- Every new data directory created inside the launcher data root must be
  handled by the data-root relocation in `internal/platform/dataroot` so it
  moves with the data folder: it must not be added to `reservedNames` (so
  `CopyData`/`TotalSize` include it), and it must be added to the directory
  list in `removeOldRoot` in `internal/platform/dataroot/dataroot.go`. The
  existing data directories are `versions`, `instances`, `downloads`,
  `cache`, `security`, `updates`, `logs`, and `backups`.
- Architecture rules are enforced by `scripts/check-architecture.sh`
  (`make architecture`, included in `make security` and CI): no Wails,
  app, or platform imports in features; no legacy
  `application`/`domain`/`infrastructure`/`presentation` packages; no
  credential fields in generated bindings.

## Vintage Story Library

- [`github.com/AmadoMuerte/vintagestory-go`](https://github.com/AmadoMuerte/vintagestory-go) is the source of truth for reusable Vintage Story integration functionality. Its packages currently cover authentication, game versions, ModDB, modpack analysis, and the public server catalog.
- Keep Waxlight-specific orchestration, persistence, credential storage, domain models, user-facing errors, installation, launching, and Wails DTOs in this repository. Access `vintagestory-go` through the thin platform adapters when its types must be mapped into Waxlight contracts.
- Before implementing Vintage Story protocol, catalog, parsing, compatibility, or analysis behavior in Waxlight, inspect the released library API and repository. Do not create a second implementation or copy library source into Waxlight.
- If Waxlight needs generic Vintage Story behavior that the library does not expose, contribute the missing capability or fix to `vintagestory-go` first, add tests there, release an appropriate library version, and then update Waxlight to consume it. A narrow temporary adapter is acceptable only when it contains Waxlight-specific mapping or orchestration, not duplicated generic implementation.

## Safety And Data

- Never put passwords, TOTP codes, pre-login tokens, session keys, or signatures in DTOs, generated bindings, logs, errors, fixtures, URLs, process arguments, environment variables, or exports.
- Production credentials use native OS storage only; do not add plaintext or in-memory fallback.
- Logins, passwords, sessions, and all credentials must live only in the protected OS credential store, and at most be written to the game's `clientsettings.json` for the duration of a running game session. Storing, using, displaying, echoing, logging, or returning them anywhere else is strictly forbidden.
- Features that copy, export, diagnose, archive, or back up an instance must remove `sessionkey`, `sessionsignature`, `playeruid`, and `playername` from its `clientsettings.json`.
- All launcher logging goes through `internal/platform/logging` (the `slog` default handler installed at startup). Never import stdlib `log` or print to stdout for diagnostics; `slog.Info/Warn/Error` is captured in the in-memory console and the exported support log. Log only the launcher's own events and errors — never game output, credentials, or account data. The logging package is framework-free and must stay independent of features, transport, and the composition root.

## Frontend

### Stack

- React 19 + TypeScript (strict) + Vite 8 + Tailwind 4 (`@tailwindcss/vite`). Router is `react-router-dom` with `HashRouter`. Radix UI primitives, `@xterm/xterm`, `i18next`.
- Server/backend data is fetched with TanStack Query v5; client-only state lives in zustand v5 stores. Entry point is `frontend/src/app/index.tsx`; the `@/` alias resolves to `frontend/src`.

### FSD layout (Feature-Sliced Design)

- `app/` — entry, `App.tsx` shell (sidebar, routes, watcher queries), `stores/` (zustand), `providers/queryClient.ts`, `styles.css`.
- `widgets/layout/` — Sidebar, SideNav, AccountSwitcher, AppToast, ErrorBanner, UpdateNotice; they read zustand stores directly, no props.
- `pages/<route>/` — one folder per route with the page and its tests next to it.
- `features/<domain>/` — feature UI: `auth`, `instances`, `mods`, `instance-package`, `install-game-version`, `operations`.
- `entities/<domain>/` — `model.ts` (types), `api.ts` (re-exports the api object), `queries.ts` (TanStack hooks).
- `shared/` — `api/` (transport, types, query keys), `ui/` (components), `lib/` (formatters), `i18n/`.
- `wailsjs/` — generated Wails bindings; never edit by hand, regenerate with `make wails-build` (the Wails CLI recreates them during the build) and refresh the API inventory with `make api-inventory`.

### Data flow principles

- **Server data goes through TanStack Query.** Never keep server data in `useState`, never prop-drill data or `refresh` callbacks into pages — pages and widgets read the shared cache through entity hooks (`useInstancesQuery`, `useAccountsQuery`, ...).
- **Query keys** are constants in `shared/api/keys.ts` (`INSTANCES_QUERY_KEY`, `ACCOUNTS_QUERY_KEY`, ...). After any mutation, refresh the cache with `queryClient.invalidateQueries({ queryKey })` — this replaces the old global `refresh()`.
- **Watcher queries** in `App.tsx` (core domains, `refetchInterval: 8000`) keep data fresh app-wide; pages consume the same cache. The settings query uses `staleTime: Infinity` (it changes only through the app itself).
- **Client-only state goes through zustand**: `useToastStore` (`notify(message, type?)` — never pass `notify` as a prop), `useAppShellStore` (launcher update notice, progress, fatalError, platform, version). Do not put server data in zustand.
- **User-facing errors**: use `errorMessage()` from `shared/api/bridge`; handle query errors in pages (Empty state + retry) rather than throwing.
- Backend calls run on the Wails bridge; a missing backend throws `BackendUnavailableError`.

### API access

- Import api objects from the specific domain module (`shared/api/accounts.ts`, `shared/api/instances.ts`, `shared/api/mods.ts`, `shared/api/mod-catalog.ts`, ...). The `shared/api/index.ts` barrel is types-only — never import runtime values from it.
- Entities re-export their api: `entities/account/api.ts` → `export { accountsApi } from "../../shared/api/accounts"`.
- New backend controller methods go into the matching `shared/api/<domain>.ts` module.
- Tests mock the specific module, e.g. `vi.mock("../../shared/api/accounts", () => ({ accountsApi: api }))`.

### UI conventions

- Components live in `shared/ui/` as individual files (`button.tsx`, `modal.tsx`, `empty.tsx`, `field.tsx`, ...). Import directly from the file, never from a barrel.
- Radix wrappers live in `shared/ui/` too (`dialog.tsx`, `select.tsx`, `dropdown-menu.tsx`, `toast.tsx`, `tooltip.tsx`, `progress.tsx`, `checkbox.tsx`, `confirm-dialog.tsx`).
- Memoize list items (`ModCard`, `InstanceCard`) with `memo` and stable `useCallback` handlers; use Map indexes instead of `.find()` in render loops.
- Follow Vercel React best practices (see `.agents/skills/vercel-react-best-practices`): no barrels, lazy-load heavy chunks (`React.lazy` for xterm/LogConsole), no waterfalls.

### Localization

- `frontend/src/shared/i18n/locales/en.json` is canonical. Translate values only; preserve keys, `{{interpolation}}`, and plural suffixes. Run `npm run check:i18n --prefix frontend`.
- Adding a language also requires registration in `frontend/src/shared/i18n/languages.ts`, `frontend/src/shared/i18n/index.ts`, and backend language validation.

### Commands

- Install: `npm ci --prefix frontend`. After any dependency change, regenerate `frontend/package.json.md5` with `md5sum frontend/package.json > frontend/package.json.md5`.
- Build (full validation: i18n check, lint, format check, tsc, vite): `npm run build --prefix frontend`. Tests: `npm test --prefix frontend`. Format: `npm run format --prefix frontend`; dev server: `npm run dev --prefix frontend`.
- Run `npm run build` and `npm test` before committing; the pre-commit hook runs `make format-check lint` and blocks on failure.

### Testing

- vitest + Testing Library; tests start with `// @vitest-environment jsdom`.
- Components using queries must be wrapped in a fresh `QueryClientProvider` per test (defaults: `retry: false`).
- Toast assertions: `useToastStore.setState({ notify: vi.fn() })` before render, then assert on the mock.
- Backend mocks target the domain module (see above). App tests use fake timers to advance the 8s watcher interval.

## GitHub Communication

- Write all GitHub content exclusively in English: pull request titles and descriptions, commit messages, review comments, issue comments, replies, and labels. Never write GitHub content in Russian or any other language.
- Normal development pull requests must target `dev`.
- Pull requests targeting `main` are reserved for `dev -> main` promotion only.
- When an agent creates a normal pull request with GitHub CLI, use `gh pr create --base dev`.
- Never bypass required CI checks or branch protection to merge a pull request.

## Localization And Releases

- Frontend localization rules live in the Frontend section above.
- Release versions must match both `wails.json` and `cmd/waxlight/wails.json`; use `make set-version VERSION=X.Y.Z` rather than changing one file.
- Releases are produced only from `main`, after `dev` has been promoted through a successful `dev -> main` pull request.
