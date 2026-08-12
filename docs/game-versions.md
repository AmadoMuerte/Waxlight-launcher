# Game version discovery and installation

Waxlight uses the official Vintage Story version feed:

```text
GET https://api.vintagestory.at/stable-unstable.json
```

The production integration was verified against the live response on
2 August 2026. The endpoint returns a JSON object keyed by game version. Each
version contains platform distributions with a filename, a human-readable file
size, an MD5 checksum, official CDN and account-area URLs, and an optional
`latest` marker.

Waxlight selects only the current client platform:

- `linux` for Linux x64, delivered as `.tar.gz`;
- `windows` for Windows x64, delivered as an Inno Setup `.exe`.

Other entries such as update installers, servers, and macOS clients are ignored.
Only HTTPS downloads from `cdn.vintagestory.at` are accepted, and the URL's
basename must match the catalog filename. Malformed releases are skipped rather
than trusted.

## Installation pipeline

The application layer performs the following sequence:

1. confirm the version is not installed or already being installed;
2. check that enough free space exists for the package and extracted files;
3. create a persisted, cancellable operation;
4. download through the shared manager, resuming a `.partial` file when the CDN
   supports HTTP ranges;
5. verify the MD5 supplied by the official feed;
6. install into the shared `versions/<version-id>` store;
7. locate the game executable and write `.waxlight-version`;
8. register the version in SQLite only after installation succeeds;
9. remove the cached package.

Linux extraction is path-traversal protected and rejects links. Windows uses
the official full installer. The current installer identifies itself as Inno
Setup 6.4.3; Waxlight invokes the documented `/VERYSILENT`, `/NORESTART`,
`/CURRENTUSER`, `/NOICONS`, and `/DIR=` switches without a shell. See the
[Inno Setup command-line documentation](https://jrsoftware.org/ishelp/topic_setupcmdline.htm).

The official feed currently publishes MD5 rather than SHA-256. MD5 is used only
to verify that a downloaded file matches the publisher's feed; it is not treated
as a cryptographic signature. Manual archive import continues to support an
optional user-supplied SHA-256 checksum.

## Known boundaries

- The official feed does not expose release timestamps, so Waxlight does not
  invent them.
- Automatic client installation currently supports x64 Linux and x64 Windows,
  matching the project's MVP platforms and the feed's available packages.
- The feed is an official production endpoint but is not accompanied by a
  published schema or stability guarantee. Its contract is isolated in
  `internal/platform/vintagestory` and covered by parser tests.
- Waxlight does not bypass game authentication. A valid Vintage Story account
  session is still required by the launch pipeline.
