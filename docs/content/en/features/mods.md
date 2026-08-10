---
title: Mods & ModDB
description: A built-in Vintage Story ModDB browser — search, install, and update mods per instance.
order: 40
---

# Mods & ModDB

A built-in Vintage Story ModDB browser: search, install, update, and manage mods per instance.

## The ModDB catalog

The Mods section talks to the official Vintage Story ModDB catalog:

- catalog search and filters;
- a mod details page with description, versions, and changelogs;
- installing a chosen mod version into a specific instance;
- batch mod installation.

## Managing an instance's mods

- install from the catalog and from local files;
- enable/disable without removal;
- update and remove;
- manually installed mods appear alongside catalog ones (marked as local).

## Update analysis

Waxlight can check which of an instance's mods can be updated through the catalog. The check (an instance's "mod updates") builds a report for every installed mod:

| Status | Meaning |
| --- | --- |
| `up_to_date` | Installed version equals the catalog candidate |
| `update_available` | A newer version exists; target ID and version are provided |
| `not_updatable` | Cannot be updated via the catalog: local mod, absent from the catalog, or catalog error |
| `unknown` | State could not be determined |

The update candidate is chosen by priority: the version the catalog marks as latest → newest stable → newest of any kind. The report additionally shows:

- **compatibility** — whether the candidate supports the instance's game version (informational; updates are never blocked);
- **added dependencies** — not yet present in the instance;
- **removed dependencies** — no longer required by the new version;
- changelogs and a prerelease flag.

> [!NOTE] A report, not an action
> The analysis downloads nothing and changes nothing in the instance. Updates are applied by an explicit user action through the regular catalog download flow.

Technical details of the analysis library: [modpack.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/modpack.md).
