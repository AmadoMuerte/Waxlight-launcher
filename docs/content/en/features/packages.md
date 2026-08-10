---
title: Instance packages
description: Export and import instances as portable packages with private-data sanitization.
order: 60
---

# Instance packages

Export and import instances as portable Waxlight packages — share builds and move them between machines.

## What a package is

A Waxlight package is an instance archive you can hand to another user or import on another computer. It contains the instance's files: mods, configs, settings — everything needed to reproduce the build.

## Private-data sanitization

On export, the package is automatically sanitized. Excluded from it:

- `sessionkey`, `sessionsignature`, `playeruid`, and `playername` from `clientsettings.json`;
- MP tokens, passwords, credential-store secrets, authentication tokens;
- other account- and machine-specific settings.

> [!TIP] Safe to share
> A package contains no credentials: the recipient needs their own Vintage Story account to play the imported build.

## Import

When importing a package, Waxlight unpacks the instance into the library. After import, select an account for the instance and, if needed, a game version — and you are ready to play.
