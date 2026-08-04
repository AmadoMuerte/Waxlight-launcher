# Contributing to Waxlight Launcher

Thanks for helping improve Waxlight. Bug fixes, tests, documentation, platform
support, and focused feature proposals are welcome.

## Development setup

You need Go 1.24 or newer, Node.js 22 or newer, npm, a C compiler, and the Wails
v2 platform dependencies for your operating system. Linux development also
requires GTK3 and WebKitGTK 4.1 development packages.

```bash
git clone https://github.com/AmadoMuerte/Waxlight-launcher.git
cd Waxlight-launcher
npm ci --prefix frontend
make test
make build
```

Installing frontend dependencies also installs Git hooks. Before each commit, the
pre-commit hook checks Go and frontend formatting and static analysis. Fix any
reported problem before committing; use `git commit --no-verify` only for an
emergency bypass.

For live Wails development, install the matching CLI and run it from the command
directory:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0
cd cmd/waxlight
wails dev
```

## Before opening a pull request

Run all required checks:

```bash
go test ./...
npm ci --prefix frontend
npm --prefix frontend test
npm --prefix frontend run build
go test -race ./...
go vet ./...
go install golang.org/x/vuln/cmd/govulncheck@v1.6.0
govulncheck ./...
./scripts/check-security-patterns.sh
```

Keep pull requests focused. Add or update tests for behavior changes and update
public documentation when user-visible behavior changes.

## Localization

English is the source and fallback language. To add a language, copy
`frontend/src/i18n/locales/en.json` to `<language-code>.json` and translate only
the values. Keep every key unchanged, including plural suffixes, and preserve
interpolation expressions such as `{{name}}`.

Then add the language to `frontend/src/i18n/languages.ts`, register its resource
in `frontend/src/i18n/index.ts`, and allow it in the backend language validation.
Run the translation check and both test suites before opening a pull request:

```bash
npm run check:i18n --prefix frontend
npm test --prefix frontend
go test ./...
```

Mention added or changed languages in the pull request description so reviewers
can verify terminology and plural forms.

## Architecture expectations

- Keep domain and application logic independent from Wails and React.
- Use Wails as a transport layer, not as the home of business rules.
- Route frontend backend calls through `frontend/src/shared/api`.
- Keep asynchronous data and business behavior outside presentational components.
- Avoid TypeScript `any` unless there is no safer boundary type.
- Preserve instance isolation and treat the filesystem as the source of truth for
  installed files.
- Never commit credentials, databases, logs, downloaded game files, generated
  release packages, or local environment files.
- Never add plaintext credential persistence or a production fallback from the
  native store. Developer-only memory stores must remain test-scoped and must
  not be selectable by release builds.
- Keep passwords, TOTP codes, pre-login tokens, session keys/signatures, and
  equivalent bearer material out of DTOs, generated bindings, logs, errors,
  fixtures, process arguments, environment variables, exports, and diagnostics.
- Any feature that copies, exports, diagnoses, archives, or backs up an instance
  must remove the four authentication properties from `clientsettings.json`.
- Preserve fixed HTTPS authentication endpoints, normal TLS verification,
  redirect rejection, body limits, and safe public error mapping.
- Add failure-injection and sentinel leakage tests for authentication changes.

## Commit and pull request workflow

1. Fork the repository.
2. Create a branch from `main`.
3. Make a focused change.
4. Add or update tests.
5. Run the required checks.
6. Commit with a concise, descriptive message.
7. Open a pull request and complete the template.

Use GitHub Issues for bugs and feature requests. Report vulnerabilities privately
as described in [SECURITY.md](SECURITY.md).
