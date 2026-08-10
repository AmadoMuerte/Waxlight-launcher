---
title: Operations & downloads
description: Track downloads and long-running work — progress, cancellation, history.
order: 70
---

# Operations & downloads

All long-running work — game version downloads and installations — on one page: progress, cancellation, history.

## Statuses

| Status | State |
| --- | --- |
| `queued` | Queued (active) |
| `running` | Running (active) |
| `completed` | Finished successfully |
| `failed` | Finished with an error |
| `cancelled` | Legacy record from older launcher versions |

## Cancellation is a rollback

A successful cancellation leaves no history entry. Waxlight:

1. cancels the operation context and waits for the downloader/installer to stop;
2. removes both `<file>.partial` and `<file>` from the `downloads/` directory;
3. deletes the operation row from the database and emits `operation:removed`;
4. releases locks and resolves the cancel call.

If filesystem cleanup fails, the operation is marked `failed` and the call returns an error — the row is kept intentionally to surface the problem and let you retry deletion after fixing permissions.

## Resumable downloads

Official game packages download to `downloads/<name>.partial`. After checksum verification the file is renamed without `.partial`. On a network error the partial file is kept — the next attempt resumes where it stopped when the CDN supports HTTP ranges.

## History

- Active operations can only be cancelled.
- Finished ones can be deleted individually (with confirmation) or cleared all at once.
- Deleting active operations is rejected at the database level: the guard holds even against a misbehaving client.
- The list is newest-first and limited to 100 records.

Page contract: [operations-page.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/operations-page.md).
