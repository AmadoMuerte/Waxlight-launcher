---
title: Repository documents
description: The complete list of important project documents — policies, license, technical and process documents.
order: 70
---

# Repository documents

The complete list of important project documents: policies, license, technical and process documents.

## Policies

| Document | Contents |
| --- | --- |
| [PRIVACY.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/PRIVACY.md) | Local data, optional telemetry (disabled by default), third-party services, contact for privacy requests. |
| [SECURITY.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/SECURITY.md) | Private vulnerability reporting, security scope, threat model, credential storage, supported versions. |
| [CODE_SIGNING_POLICY.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/CODE_SIGNING_POLICY.md) | Current Windows signing status, release provenance, and artifact verification. |

The same policies in a readable form: [Privacy](./policies/privacy.md), [Security](./policies/security.md), [Code signing](./policies/code-signing.md).

## License and notices

| Document | Contents |
| --- | --- |
| [LICENSE](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/LICENSE) | GNU GPL v3.0: you may use, study, modify, and redistribute under the terms of GPLv3. |
| [NOTICE](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/NOTICE) | Copyright (© 2026 AmadoMuerte), redistribution obligations, third-party component notices (go-keyring, dbus, wincred — MIT/permissive). |

## For users

| Document | Contents |
| --- | --- |
| [README.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/README.md) | Project overview, features, download, first steps, building from source. Russian version: docs/README.ru.md. |
| [AGENTS.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/AGENTS.md) | Project rules for agents and contributors: workflow, layer structure, data safety, branching (main/dev), PR requirements. |

## Technical documents

| Document | Contents |
| --- | --- |
| [authentication.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/authentication.md) | Vintage Story authentication protocol, network boundary, session storage, transactions, legacy-secret migration, launch credential lifetime. |
| [game-versions.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/game-versions.md) | The official version feed, platform selection, installation pipeline, MD5/SHA-256, known boundaries. |
| [modpack.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/modpack.md) | The mod update analysis library: Catalog contract, statuses, candidate selection, compatibility, dependencies. |
| [operations-page.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/operations-page.md) | Operations page contract: statuses, cancellation as rollback, history deletion, required regression tests. |
| [windows-updater.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/windows-updater.md) | Windows auto-update architecture: SHA256SUMS vs Authenticode, trust model, and installation modes. |

## For contributors

| Document | Contents |
| --- | --- |
| [CONTRIBUTING.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/CONTRIBUTING.md) | Environment setup, required checks before a PR, localization, architecture expectations, commit workflow. |

A developer summary is on the [Development](./development.md) page.
