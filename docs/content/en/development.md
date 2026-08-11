---
title: Development
description: Building from source, architecture, tests, and how to contribute.
order: 80
---

# Development

Building from source, architecture, tests, and how to contribute.

## Stack

- **Backend:** Go 1.25+, Wails 2.11, SQLite.
- **Frontend:** React 19, TypeScript (strict), Vite 8, Tailwind 4, TanStack Query v5, zustand v5, Radix UI, i18next.
- **Platforms:** Windows x64, Linux x64 (GTK3 + WebKitGTK 4.1).

## Building from source

Requirements: Go 1.25+, Node.js 22+, Wails 2.11, a C compiler, and the [Wails platform dependencies](https://wails.io/docs/gettingstarted/installation/).

```bash
git clone https://github.com/AmadoMuerte/Waxlight-launcher.git
cd Waxlight-launcher
npm ci --include=dev --prefix frontend
go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0
cd cmd/waxlight
wails dev
```

Production build from the repository root:

```bash
make wails-build
```

> [!NOTE] Wails builds only
> `wails dev` runs only from `cmd/waxlight`. Supported desktop artifacts are built with `make wails-build`; a plain `go build` without Wails desktop tags is not a supported GUI build.

## Architecture

The Go code is layered:

| Layer | Purpose |
| --- | --- |
| `internal/domain` | Models and errors |
| `internal/application` | Use cases and ports |
| `internal/infrastructure` | Adapters: database, downloader, credential store, filesystem, mod catalog, etc. |
| `internal/presentation` | Wails controllers and DTOs |

`cmd/waxlight/main.go` starts Wails and bootstrap; domain and application logic stay independent of Wails and React. The frontend follows Feature-Sliced Design (`app/`, `pages/`, `features/`, `entities/`, `shared/`, `widgets/`); backend calls go only through `frontend/src/shared/api`.

## Checks and tests

```bash
make test           # frontend production build (i18n + tsc), then Go and frontend tests
make format         # gofmt + oxfmt
make lint           # Go static analysis (Linux target) + oxlint
make vet            # go vet for the current platform
make security       # prohibited-pattern and vulnerability checks
make release-check VERSION=X.Y.Z  # full release validation
```

Focused Go tests: `go test ./path/to/package -run TestName`. The pre-commit hook runs `make format-check lint` and blocks commits on failure.

## Branching and PRs

- `main` is the stable production branch; `dev` is the integration branch.
- Working branches start from `dev` with prefixes like `feat/`, `fix/`, `refactor/`, `chore/`, `docs/`, `test/`, `ci/`.
- Normal PRs target `dev`; only the `dev → main` promotion PR may target `main`.
- Direct commits to `main` and `dev` are forbidden, as is force-pushing them.

## Security rules for contributors

- Never put passwords, TOTP codes, pre-login tokens, session keys, or signatures in DTOs, generated bindings, logs, errors, fixtures, URLs, process arguments, environment variables, or exports.
- Production credentials use native OS storage only; no plaintext or in-memory fallback.
- Features that copy/export/diagnose/archive instances must remove the four authentication properties from `clientsettings.json`.
- Logging goes only through `internal/platform/logging` (slog); stdlib `log` and stdout prints are forbidden.

## Localization

The canonical locale is `frontend/src/shared/i18n/locales/en.json`: translate values only; preserve keys, `{{...}}` interpolations, and plural suffixes. Validation: `npm run check:i18n --prefix frontend`. Adding a language also requires registration in `languages.ts`, `i18n/index.ts`, and backend validation. The interface is already available in 10 languages: English, Русский, Беларуская, Español, Français, Deutsch, Қазақша, Polski, Svenska, Português.

## How to help

Code, translations, testing, documentation, bug reports, and focused feature proposals are welcome. Before opening a PR, read [CONTRIBUTING.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/CONTRIBUTING.md). Report vulnerabilities privately via the [security policy](./policies/security.md). Questions and discussions — on [Discord](https://discord.gg/CrRHvg9UVw).
