# Waxlight Launcher

Waxlight Launcher is a modern unofficial launcher for Vintage Story.

> Waxlight Launcher is not affiliated with or endorsed by the developers of Vintage Story.

## MVP

The current MVP provides a Wails v2 desktop shell with a React/TypeScript interface and a Go/SQLite core:

- isolated game instances;
- browsing and installing multiple versions from the official Vintage Story
  release catalog, with local archive import as a fallback;
- official Vintage Story account login, including TOTP/2FA;
- multiple accounts with global and per-instance selection;
- local mod installation, enable/disable and removal;
- game process launch/stop, logs and play-session tracking;
- total and per-instance playtime;
- operation history and basic settings;
- Linux and Windows process implementations.

Waxlight uses Vintage Story's currently available, but publicly undocumented,
authentication endpoints. The protocol is isolated behind a Go client because it
may change. Passwords and TOTP codes are never persisted or sent to React;
session credentials are treated as secrets. See
[the authentication notes](docs/authentication.md) for the protocol, storage,
and launch-pipeline details.

Game releases are discovered through Vintage Story's official version feed and
downloaded from its official CDN. Downloads are resumable, checksum-verified,
cancellable, and shared by the Linux and Windows installation flows. See
[the game version notes](docs/game-versions.md) for the confirmed contract and
platform details.

## Development

Requirements: Go 1.24+, Node.js 22+, a C compiler for SQLite, Wails v2 platform dependencies.

```bash
npm --prefix frontend install
make test
make build
./build/waxlight
```

`make build` adds the required Wails production tags. A plain `go build` creates a stub that exits with “correct build tags”, so it must not be used for the desktop binary.

For the usual live-reload workflow, install the Wails v2 CLI. Linux builds use
the installed WebKitGTK 4.1 development package through the `webkit2_41` tag.

Wails v2.11 generates bindings from the current package directory. With
Waxlight's `cmd/waxlight` layout, run `wails dev` or `wails build` from that
directory; `make wails-build` is the repository-root shortcut. A matching
configuration beside `main.go` keeps generated bindings in
`frontend/src/wailsjs`.

Application data is stored under the operating system user config directory in
`waxlight/`. Account metadata lives in SQLite. Session keys currently use a
separate atomic `account-secrets.json` fallback with owner-only permissions on
POSIX systems; migration to Secret Service on Linux and Credential Manager on
Windows remains planned. Long-running downloads use one context-aware manager
with a default limit of three concurrent transfers.
