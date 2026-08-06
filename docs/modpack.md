# Mod update analysis library

`internal/modpack` is a standalone library that answers one question: **for a
given instance build, which installed mods can be updated through the mod
catalog, which cannot, and why**.

It is deliberately independent of the rest of Waxlight. It does not import the
launcher's database, application services, downloader, HTTP clients, Wails
controllers, or React frontend. It defines its own input types and a single
`Catalog` port, so it can be unit-tested with a fake catalog and reused without
the desktop application.

## What it does and does not do

- **Does**: analyze a build and produce an update *report*.
- **Does not**: download mods, install files, modify the instance, change game
  versions, or touch `Saves/`, `config/`, `groups.json`, or any other user
  data. Applying an update is the caller's job.

## Dependencies

The package imports only the Go standard library and `golang.org/x/mod/semver`
(already a project dependency). It has no imports from `internal/...`.

## The contract

```go
type Build struct {
    GameVersion string        // e.g. "1.19.8"
    Mods        []ModInstall
}

type ModInstall struct {
    ModID        string   // catalog id; empty for manually installed mods
    Name         string
    Version      string   // installed version
    Managed      bool     // installed from the catalog
    FileName     string
    Enabled      bool
    Dependencies []string // mod IDs declared by the installed version
}

type Catalog interface {
    Get(ctx context.Context, modID string) (ModInfo, error)
}
```

A `Catalog` implementation returns `ModInfo` for a mod: its `ID`, the
`LatestVersion` string, and its published `Versions` (each with `ID`, `Version`,
`ReleaseType`, `GameVersions`, `Changelog`, and `Dependencies`). The contract
for an unknown mod is an empty `ModInfo` (empty `ID`); any other error is
treated as a per-mod catalog failure.

`Analyze(ctx, build, catalog)` returns:

```go
type Report struct {
    Build   Build
    Mods    []ModUpdate
    Summary Summary
}
```

One `ModUpdate` is produced per installed mod. `Summary` counts `TotalMods`,
`UpToDate`, `UpdatesAvailable`, `NotUpdatableLocal`, `NotUpdatableAbsent`,
`NotUpdatableCatalogError`, and `Incompatible`.

## Statuses and reasons

| Status | Meaning | Reason (for `not_updatable`) |
| --- | --- | --- |
| `up_to_date` | Installed version equals the catalog candidate | — |
| `update_available` | A newer version exists; `TargetVersionID`/`TargetVersion` are set | — |
| `not_updatable` | The mod cannot be updated through the catalog | `local_mod`, `not_in_catalog`, `catalog_error` |
| `unknown` | State could not be determined | — |

- `local_mod`: the mod was installed manually (not `Managed`, or no `ModID`).
- `not_in_catalog`: `Get` returned an empty `ModInfo`.
- `catalog_error`: `Get` returned an error. This is recorded per mod and does
  **not** abort the rest of the report; a canceled context is the only thing
  that aborts the whole analysis.

## Update candidate selection

For each catalog mod the library prefers, in order:

1. the version the catalog marks as `LatestVersion`;
2. the newest stable release;
3. the newest release of any kind.

`Prerelease` is set when the selected release is not stable. The target is
reported only when it differs from the installed version (compared with
`VersionEquals`, see below).

## Compatibility

`Compatible` reports whether the candidate's `GameVersions` include the build's
game version. A version entry such as `1.19` also covers every `1.19.x`
release; an empty supported list means the candidate does not restrict
compatibility. Compatibility is *information* — the library never blocks
anything.

## Dependency changes

`AddedDeps` are dependencies of the candidate that are not installed in the
build and not already declared by the installed version (built-in game modules
`game`, `survival`, `creative` are always ignored). `RemovedDeps` are
dependencies declared by the installed version that the candidate no longer
requires and that are still installed; such mods may become unused after the
update.

Because the catalog exposes dependencies only for its current release, `AddedDeps`
is populated only when the `Catalog` implementation supplies them; `RemovedDeps`
is computed from the installed version's declared `Dependencies`, which the
caller reads from the installed mod's `modinfo.json`.

## Version comparison

Versions are compared as semantic versions when both parse (`1.2.3` equals
`v1.2.3`), with a case-insensitive string fallback otherwise. Unparseable
versions sort before parseable ones.

## Integration with the launcher

The application layer adapts the launcher's `ModCatalog` port to `modpack.Catalog`
(`internal/application/instance_updates.go`). `Service.CheckInstanceModUpdates`
loads an instance, its game version, and its installed mods, maps them into a
`Build` (extracting catalog IDs from `moddb:<id>:<version>` sources and
dependencies from each installed `modinfo.json`), and calls `Analyze`. A
`not_found` catalog error is translated to an empty `ModInfo` so unknown mods
are classified as `not_in_catalog` rather than `catalog_error`.

The report reaches the UI through a Wails controller and is rendered by
`ModUpdatesModal` (badges, changelogs, compatibility and dependency warnings,
and an explicit "Update" action that calls the existing catalog download flow).

## Testing

The library has no external dependencies in its tests: `analyze_test.go` and
`compare_test.go` use a fake `Catalog` and cover every status, candidate
selection, compatibility, dependency-diff, and cancellation path. Run them with:

```bash
go test ./internal/modpack/
```

When changing the library, keep the package free of imports from other Waxlight
packages so these guarantees stay enforceable.
