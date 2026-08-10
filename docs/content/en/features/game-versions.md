---
title: Game versions
description: Multiple Vintage Story versions side by side, installed from the official version feed.
order: 20
---

# Game versions

Multiple Vintage Story versions installed side by side, from the game's official version feed.

## Where versions come from

Waxlight uses the official Vintage Story version feed:

```text
GET https://api.vintagestory.at/stable-unstable.json
```

For each version, the feed lists platform distributions with a filename, human-readable size, MD5 checksum, and official URLs. Waxlight selects only the current client platform:

- **Linux x64** — a `.tar.gz` archive;
- **Windows x64** — the official Inno Setup `.exe` installer.

Servers, macOS clients, and other entries are ignored. Only HTTPS downloads from `cdn.vintagestory.at` are accepted, and the URL basename must match the catalog filename; malformed releases are skipped rather than trusted.

## Installing a version

The Game Versions section lists installed versions and those available for download (stable and unstable, with a filter). The installation pipeline:

1. confirm the version is not installed or already being installed;
2. check free space for the package and extracted files;
3. create a persisted, cancellable operation (visible on the [Operations](./operations.md) page);
4. download through the shared manager, resuming a `.partial` file when the CDN supports HTTP ranges;
5. verify the MD5 from the official feed;
6. install into the shared `versions/<version-id>` store;
7. locate the game executable and write `.waxlight-version`;
8. register the version in SQLite only after installation succeeds;
9. remove the cached package.

Linux extraction is path-traversal protected and rejects links. Windows uses the official installer with the documented `/VERYSILENT /NORESTART /CURRENTUSER /NOICONS /DIR=` switches, without a shell.

> [!NOTE] About MD5
> The official feed publishes MD5, not SHA-256. MD5 is used only to verify that a downloaded file matches the publisher's feed; it is not treated as a cryptographic signature. Manual archive import supports an optional user-supplied SHA-256.

## Install from file

Besides the official feed, a version can be installed from a local archive/file ("Install from file") with optional SHA-256 verification. Useful for versions not present in the feed.

## Known boundaries

- The feed does not publish release timestamps, so Waxlight does not invent them.
- Automatic installation supports x64 Linux and x64 Windows.
- The feed is an official production endpoint without a published schema or stability guarantee; its parser is isolated and covered by tests.
- Waxlight does not bypass game authentication: a valid Vintage Story account session is still required to play.

Technical document: [game-versions.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/game-versions.md).
