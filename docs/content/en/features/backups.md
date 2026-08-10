---
title: Backups & recovery
description: Snapshots, safety backups before changes, and restoration of a known-good state.
order: 50
---

# Backups & recovery

Snapshots, safety backups before risky changes, and a return to a known-good state.

## Snapshots

Waxlight can create instance snapshots — saved copies of an instance's state you can return to after unsuccessful experiments with mods or settings. Snapshots are stored in the `backups/` directory inside the launcher's data folder.

## Safety backups

Before potentially destructive changes to an instance, the launcher automatically creates a safety snapshot, so there is always a rollback point.

## Last known good

The last-known-good mechanism records the configuration in which the game last launched successfully and can restore it after failures — for example, when an instance stops launching after mod updates.

> [!WARNING] Stop the game before backing up
> While the game is running, the account session data sits in `clientsettings.json`. A copy taken at that moment may capture it. Stop the game before creating backups and filesystem snapshots.

## Export safety

Every feature that copies, exports, or archives an instance removes `sessionkey`, `sessionsignature`, `playeruid`, and `playername` from its `clientsettings.json`. This is a hard project rule — see [Instance packages](./packages.md) and [Security](../policies/security.md).
