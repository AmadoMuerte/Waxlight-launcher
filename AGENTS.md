# Waxlight Launcher

## Workflow

- Frontend lives in `frontend/`; install its locked dependencies with `npm ci --include=dev --prefix frontend`. This also registers the Husky pre-commit hook from `frontend/.husky`.
- `make test` first runs the frontend production build (including i18n and TypeScript checks), then Go and frontend tests.
- Run focused Go tests with `go test ./path/to/package -run TestName`.
- Run `make format` to format Go and frontend sources. Use `make format-check` to validate `gofmt` and `oxfmt --check` without changing files.
- Run `make lint` for Linux-targeted Go static analysis and frontend `oxlint`; run `make vet` for Go static analysis on the current platform. Run `make security` for prohibited-pattern and vulnerability checks.
- Pre-commit runs `make format-check lint` and blocks commits on failure. Use `git commit --no-verify` only for an emergency bypass.
- Full release validation requires a version: `make release-check VERSION=X.Y.Z`.
- `make release VERSION=X.Y.Z` requires clean, synchronized `main` and then commits version files, pushes `main`, and creates and pushes a release tag.
- Run `wails dev` only from `cmd/waxlight`. Build supported desktop artifacts with `make wails-build`; plain `go build` without Wails desktop tags is not a supported GUI build.
- Native credential-store integration tests are tagged and require Linux Secret Service or Windows Credential Manager; CI provides those platform services.

## Structure

- `cmd/waxlight/main.go` starts Wails and bootstraps `internal/bootstrap`; keep domain/application logic independent of Wails and React.
- Go layers: `internal/domain` models/errors, `internal/application` use cases and ports, `internal/infrastructure` adapters, `internal/presentation` Wails controllers and DTOs.
- Frontend backend access belongs in `frontend/src/shared/api`; Wails bindings are generated under `frontend/src/wailsjs`.
- Runtime SQLite schema changes belong in `internal/infrastructure/database/sqlite.go`; files in `migrations/` are not canonical runtime migrations.
- Every new data directory created inside the launcher data root must be handled by the data-root relocation in `internal/infrastructure/dataroot` so it moves with the data folder: it must not be added to `reservedNames` (so `CopyData`/`TotalSize` include it), and it must be added to the directory list in `removeOldRoot` in `internal/infrastructure/dataroot/dataroot.go`. The existing data directories are `versions`, `instances`, `downloads`, `cache`, `security`, `updates`, `logs`, and `backups`.

## Safety And Data

- Never put passwords, TOTP codes, pre-login tokens, session keys, or signatures in DTOs, generated bindings, logs, errors, fixtures, URLs, process arguments, environment variables, or exports.
- Production credentials use native OS storage only; do not add plaintext or in-memory fallback.
- Logins, passwords, sessions, and all credentials must live only in the protected OS credential store, and at most be written to the game's `clientsettings.json` for the duration of a running game session. Storing, using, displaying, echoing, logging, or returning them anywhere else is strictly forbidden.
- Features that copy, export, diagnose, archive, or back up an instance must remove `sessionkey`, `sessionsignature`, `playeruid`, and `playername` from its `clientsettings.json`.
- All launcher logging goes through `internal/infrastructure/logging` (the `slog` default handler installed at startup). Never import stdlib `log` or print to stdout for diagnostics; `slog.Info/Warn/Error` is captured in the in-memory console and the exported support log. Log only the launcher's own events and errors — never game output, credentials, or account data. The logging package is framework-free and must stay independent of domain, application, and presentation.

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
- `wailsjs/` — generated Wails bindings; never edit by hand, regenerate with `wails generate`.

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

## Localization And Releases

- Frontend localization rules live in the Frontend section above.
- Release versions must match both `wails.json` and `cmd/waxlight/wails.json`; use `make set-version VERSION=X.Y.Z` rather than changing one file.
