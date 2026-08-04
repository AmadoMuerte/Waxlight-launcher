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

English is the source and fallback language. All translation keys must remain synchronized with the English locale.

To add a new language:

1. Add the language code and its display name to the root [`languages.json`](./languages.json) file.

2. Copy the English locale:

   ```bash
   cp frontend/src/i18n/locales/en.json frontend/src/i18n/locales/<language-code>.json
   ```

3. Translate the values in the new locale file.

   Locale files are stored in:

   ```text
   frontend/src/i18n/locales/
   ```

When editing a locale:

* translate values only;
* do not rename, remove, or add translation keys independently;
* preserve interpolation expressions such as `{{name}}`;
* preserve plural suffixes such as `_one`, `_few`, `_many`, and `_other`;
* keep paths, URLs, technical names, and version identifiers unchanged;
* use the same JSON structure as `en.json`.

After adding or updating a locale, build the frontend:

```bash
npm --prefix frontend run build
```

Then run the localization validation and tests:

```bash
npm run check:i18n --prefix frontend
npm test --prefix frontend
go test ./...
```

Do not register locales manually in TypeScript or Go code. The supported languages are generated from `languages.json` during the build process.

Mention all added or changed languages in the pull request description so reviewers can verify terminology, formatting, interpolation expressions, and plural forms.


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
