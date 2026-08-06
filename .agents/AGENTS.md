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
- Every new data directory created inside the launcher data root must be handled by the data-root relocation in `internal/infrastructure/dataroot` so it moves with the data folder: it must not be added to `reservedNames` (so `CopyData`/`TotalSize` include it), and it must be added to the directory list in `removeOldRoot` in `internal/infrastructure/dataroot/dataroot.go`. The existing data directories are `versions`, `instances`, `downloads`, `cache`, `security`, `updates`, and `logs`.

## Safety And Data

- Never put passwords, TOTP codes, pre-login tokens, session keys, or signatures in DTOs, generated bindings, logs, errors, fixtures, URLs, process arguments, environment variables, or exports.
- Production credentials use native OS storage only; do not add plaintext or in-memory fallback.
- Logins, passwords, sessions, and all credentials must live only in the protected OS credential store, and at most be written to the game's `clientsettings.json` for the duration of a running game session. Storing, using, displaying, echoing, logging, or returning them anywhere else is strictly forbidden.
- Features that copy, export, diagnose, archive, or back up an instance must remove `sessionkey`, `sessionsignature`, `playeruid`, and `playername` from its `clientsettings.json`.
- All launcher logging goes through `internal/infrastructure/logging` (the `slog` default handler installed at startup). Never import stdlib `log` or print to stdout for diagnostics; `slog.Info/Warn/Error` is captured in the in-memory console and the exported support log. Log only the launcher's own events and errors — never game output, credentials, or account data. The logging package is framework-free and must stay independent of domain, application, and presentation.

## Localization And Releases

- `frontend/src/i18n/locales/en.json` is canonical. Translate values only; preserve keys, `{{interpolation}}`, and plural suffixes. Run `npm run check:i18n --prefix frontend`.
- Adding a language also requires registration in `frontend/src/i18n/languages.ts`, `frontend/src/i18n/index.ts`, and backend language validation.
- Release versions must match both `wails.json` and `cmd/waxlight/wails.json`; use `make set-version VERSION=X.Y.Z` rather than changing one file.
