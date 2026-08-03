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
go vet ./...
npm ci --prefix frontend
npm --prefix frontend test
npm --prefix frontend run build
```

Keep pull requests focused. Add or update tests for behavior changes and update
public documentation when user-visible behavior changes.

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
