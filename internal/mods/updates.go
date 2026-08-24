package mods

import (
	"context"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/snapshots"
	"log/slog"
	"strconv"
	"strings"

	vsmodpack "github.com/AmadoMuerte/vintagestory-go/modpack"
)

// ModUpdateReport describes the update state of every installed mod of an
// instance, produced by the vintagestory-go modpack analyzer.
type ModUpdateReport = vsmodpack.Report

// modpackCatalogAdapter adapts the mods Catalog port to the standalone
// vintagestory-go/modpack Catalog interface. It translates mod metadata into
// the library's own types so the library stays independent of the launcher.
type modpackCatalogAdapter struct {
	catalog Catalog
}

func (adapter modpackCatalogAdapter) Get(
	ctx context.Context,
	modID string,
) (vsmodpack.ModInfo, error) {
	details, err := adapter.catalog.Get(ctx, modID)
	if err != nil {
		// The library's contract treats an unknown mod as an empty ModInfo.
		if isAppErrorCode(err, ErrModNotFound) {
			return vsmodpack.ModInfo{}, nil
		}
		return vsmodpack.ModInfo{}, err
	}
	info := vsmodpack.ModInfo{
		ID:            canonicalCatalogModID(details),
		LatestVersion: details.LatestVersion,
		Versions:      make([]vsmodpack.ModVersion, 0, len(details.Versions)),
	}
	latestIndex := -1
	for index, version := range details.Versions {
		if latestIndex < 0 && version.Version != "" && version.Version == details.LatestVersion {
			latestIndex = index
		}
		info.Versions = append(info.Versions, vsmodpack.ModVersion{
			ID:           version.ID,
			Version:      version.Version,
			ReleaseType:  version.ReleaseType,
			GameVersions: version.GameVersions,
			Changelog:    version.Changelog,
		})
	}
	// The catalog reports dependencies for its current release. Attach them to
	// the matching version so the library can surface dependency changes.
	if latestIndex >= 0 {
		info.Versions[latestIndex].Dependencies = modDependencies(details.Dependencies)
	}
	return info, nil
}

func modDependencies(items []ModDependency) []vsmodpack.Dependency {
	dependencies := make([]vsmodpack.Dependency, 0, len(items))
	for _, item := range items {
		dependencies = append(dependencies, vsmodpack.Dependency{
			ModID:       item.ModID,
			Name:        item.Name,
			Requirement: item.Version,
		})
	}
	return dependencies
}

// CheckInstanceModUpdates analyzes the mods installed in an instance and
// reports which of them can be updated through the mod catalog. It only
// produces information; applying an update is a separate catalog download.
func (service *CatalogService) CheckInstanceModUpdates(
	ctx context.Context,
	instanceID string,
) (ModUpdateReport, error) {
	instance, err := service.repository.GetInstance(ctx, instanceID)
	if err != nil {
		return ModUpdateReport{}, err
	}
	gameVersion := ""
	if version, versionErr := service.versions.Get(ctx, instance.GameVersionID); versionErr == nil {
		gameVersion = version.Name
		if gameVersion == "" {
			gameVersion = version.ID
		}
	}
	mods, err := service.lister.ListMods(ctx, instanceID)
	if err != nil {
		return ModUpdateReport{}, err
	}
	build := vsmodpack.Build{
		GameVersion: gameVersion,
		Mods:        make([]vsmodpack.ModInstall, 0, len(mods)),
	}
	for _, mod := range mods {
		install := vsmodpack.ModInstall{
			Name:     mod.Name,
			Version:  mod.Version,
			FileName: mod.FileName,
			Enabled:  mod.Enabled,
		}
		if modID, _, ok := ParseModDBSource(mod.Source); ok {
			install.ModID = modID
			install.Managed = true
		}
		if info, infoErr := ReadModArchiveInfo(mod.FilePath); infoErr == nil {
			for dependencyID := range info.Dependencies {
				install.Dependencies = append(install.Dependencies, dependencyID)
			}
		}
		build.Mods = append(build.Mods, install)
	}
	report, err := vsmodpack.Analyze(ctx, build, modpackCatalogAdapter{catalog: service.catalog})
	if err != nil {
		return report, err
	}
	applyUpdatePolicies(ctx, &report, mods, gameVersion, service.catalog)
	slog.Info("mod updates checked", "instance", instance.Name, "updates", report.Summary.UpdatesAvailable)
	return report, nil
}

// UpdateInstanceMods updates several installed mods of one instance in a
// single destructive transaction. It first determines which requested releases
// would actually change the instance and, when at least one update applies,
// creates exactly one automatic safety snapshot before any mod is replaced.
// The snapshot is created and completed before the first update starts; a
// failed snapshot aborts the whole operation without touching the instance.
func (service *CatalogService) UpdateInstanceMods(
	ctx context.Context,
	instanceID string,
	targets []ModUpdateTarget,
	allowIncompatible bool,
) (ModUpdateResult, error) {
	result := ModUpdateResult{}
	if err := service.gate.Begin(); err != nil {
		return result, err
	}
	defer service.gate.End()
	if len(targets) == 0 {
		return result, nil
	}
	instance, err := service.repository.GetInstance(ctx, instanceID)
	if err != nil {
		return result, err
	}
	installed, err := service.repository.ListMods(ctx, instanceID)
	if err != nil {
		return result, err
	}
	instanceVersion, err := service.versions.Get(ctx, instance.GameVersionID)
	if err != nil {
		return result, err
	}
	gameVersion := instanceVersion.Name
	if gameVersion == "" {
		gameVersion = instanceVersion.ID
	}
	pending, skipped, err := service.pendingModUpdates(ctx, installed, targets, gameVersion)
	if err != nil {
		return result, err
	}
	result.SkippedByPolicy = skipped
	if len(pending) == 0 {
		return result, nil
	}

	instanceRelease, err := service.lockInstanceMutations(instanceID)
	if err != nil {
		return result, err
	}
	defer instanceRelease()

	if err := service.snapshotter.Create(ctx, instanceID, snapshots.ReasonBeforeModUpdate, map[string]string{
		"affectedMods": strconv.Itoa(len(pending)),
	}); err != nil {
		return result, err
	}

	for _, target := range pending {
		downloadResult, err := service.downloadCatalogMod(ctx, DownloadModRequest{
			ModID:             target.ModID,
			VersionID:         target.VersionID,
			InstanceIDs:       []string{instance.ID},
			AllowIncompatible: allowIncompatible,
		}, map[string]struct{}{instance.ID: {}})
		if err != nil {
			return result, err
		}
		if len(downloadResult.Installations) > 0 && downloadResult.Installations[0].Installed {
			result.Updated++
		}
	}
	slog.Info("instance mods updated", "instance", instance.Name, "updated", result.Updated)
	return result, nil
}

// pendingModUpdates filters the requested targets to the ones that would
// actually change the instance: releases that are not installed yet or whose
// installed record points at another release. Duplicate targets are collapsed.
func (service *CatalogService) pendingModUpdates(ctx context.Context, installed []InstalledMod, targets []ModUpdateTarget, gameVersion string) ([]ModUpdateTarget, int, error) {
	installedSource := make(map[string]string, len(installed))
	policies := make(map[string]UpdatePolicy, len(installed))
	for _, mod := range installed {
		if modID, _, ok := ParseModDBSource(mod.Source); ok {
			installedSource[modID] = mod.Source
			policies[modID] = NormalizeUpdatePolicy(mod.UpdatePolicy)
		}
	}
	pending := make([]ModUpdateTarget, 0, len(targets))
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		modID := strings.TrimSpace(target.ModID)
		versionID := strings.TrimSpace(target.VersionID)
		if modID == "" || versionID == "" {
			continue
		}
		key := modID + ":" + versionID
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		if policies[modID] == UpdatePolicyPinned {
			continue
		}
		if policies[modID] == UpdatePolicyCompatibleOnly {
			details, err := service.catalog.Get(ctx, modID)
			if err != nil {
				return nil, 0, err
			}
			version, ok := FindModVersion(details.Versions, versionID)
			if !ok || !ModSupportsVersion(version.GameVersions, gameVersion) {
				continue
			}
		}
		if installedSource[modID] == ModDBSource(modID, versionID) {
			continue
		}
		pending = append(pending, ModUpdateTarget{ModID: modID, VersionID: versionID})
	}
	skipped := 0
	for modID, policy := range policies {
		if policy != UpdatePolicyPinned {
			continue
		}
		for _, target := range targets {
			if strings.TrimSpace(target.ModID) == modID && installedSource[modID] != ModDBSource(modID, strings.TrimSpace(target.VersionID)) {
				skipped++
				break
			}
		}
	}
	return pending, skipped, nil
}

func applyUpdatePolicies(ctx context.Context, report *ModUpdateReport, installed []InstalledMod, gameVersion string, catalog Catalog) {
	policies := make(map[string]UpdatePolicy, len(installed))
	for _, mod := range installed {
		if modID, _, ok := ParseModDBSource(mod.Source); ok {
			policies[modID] = NormalizeUpdatePolicy(mod.UpdatePolicy)
		}
	}
	for index := range report.Mods {
		update := &report.Mods[index]
		switch policies[update.ModID] {
		case UpdatePolicyPinned:
			if update.Status == vsmodpack.StatusUpdateAvailable {
				report.Summary.UpdatesAvailable--
				report.Summary.UpToDate++
			}
			update.Status = vsmodpack.StatusUpToDate
			update.TargetVersionID = ""
			update.TargetVersion = ""
		case UpdatePolicyCompatibleOnly:
			if update.Status != vsmodpack.StatusUpdateAvailable || update.Compatible {
				continue
			}
			details, err := catalog.Get(ctx, update.ModID)
			if err != nil {
				update.Status = vsmodpack.StatusNotUpdatable
				update.Reason = vsmodpack.ReasonCatalogError
				update.TargetVersionID = ""
				update.TargetVersion = ""
				continue
			}
			version, found := bestSatisfyingVersion(details.Versions, "*", []string{gameVersion}, false)
			if found && version.Version != update.InstalledVersion {
				update.TargetVersionID = version.ID
				update.TargetVersion = version.Version
				update.Compatible = true
				update.Changelog = version.Changelog
			}
			if !update.Compatible {
				report.Summary.UpdatesAvailable--
				report.Summary.Incompatible++
				update.Status = vsmodpack.StatusUpToDate
				update.TargetVersionID = ""
				update.TargetVersion = ""
			}
		}
	}
	report.Summary = summarizeModUpdates(report.Mods)
}

func summarizeModUpdates(updates []vsmodpack.ModUpdate) vsmodpack.Summary {
	summary := vsmodpack.Summary{TotalMods: len(updates)}
	for _, update := range updates {
		switch update.Status {
		case vsmodpack.StatusUpToDate:
			summary.UpToDate++
		case vsmodpack.StatusUpdateAvailable:
			summary.UpdatesAvailable++
			if !update.Compatible {
				summary.Incompatible++
			}
		case vsmodpack.StatusNotUpdatable:
			switch update.Reason {
			case vsmodpack.ReasonLocalMod:
				summary.NotUpdatableLocal++
			case vsmodpack.ReasonNotInCatalog:
				summary.NotUpdatableAbsent++
			case vsmodpack.ReasonCatalogError:
				summary.NotUpdatableCatalogError++
			}
		}
	}
	return summary
}
