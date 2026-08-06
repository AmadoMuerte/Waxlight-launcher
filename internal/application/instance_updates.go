package application

import (
	"context"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"github.com/waxlight/waxlight-launcher/internal/modpack"
)

// modpackCatalogAdapter adapts the application's ModCatalog port to the
// standalone modpack.Catalog interface. It translates domain mod metadata into
// the library's own types so the library stays independent of the launcher.
type modpackCatalogAdapter struct {
	catalog ModCatalog
}

func (adapter modpackCatalogAdapter) Get(
	ctx context.Context,
	modID string,
) (modpack.ModInfo, error) {
	details, err := adapter.catalog.Get(ctx, modID)
	if err != nil {
		// The library's contract treats an unknown mod as an empty ModInfo.
		if isAppErrorCode(err, domain.ErrModNotFound) {
			return modpack.ModInfo{}, nil
		}
		return modpack.ModInfo{}, err
	}
	info := modpack.ModInfo{
		ID:            canonicalCatalogModID(details),
		LatestVersion: details.LatestVersion,
		Versions:      make([]modpack.ModVersion, 0, len(details.Versions)),
	}
	latestIndex := -1
	for index, version := range details.Versions {
		if latestIndex < 0 && version.Version != "" && version.Version == details.LatestVersion {
			latestIndex = index
		}
		info.Versions = append(info.Versions, modpack.ModVersion{
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
		info.Versions[latestIndex].Dependencies = dependencies(details.Dependencies)
	}
	return info, nil
}

func dependencies(items []domain.ModDependency) []modpack.Dependency {
	dependencies := make([]modpack.Dependency, 0, len(items))
	for _, item := range items {
		dependencies = append(dependencies, modpack.Dependency{
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
func (s *Service) CheckInstanceModUpdates(
	ctx context.Context,
	instanceID string,
) (modpack.Report, error) {
	if s.modCatalog == nil {
		return modpack.Report{}, domain.NewError(
			domain.ErrModCatalog,
			"The mod catalog is not configured",
		)
	}
	instance, err := s.store.GetInstance(ctx, instanceID)
	if err != nil {
		return modpack.Report{}, err
	}
	gameVersion := ""
	if version, versionErr := s.store.GetVersion(ctx, instance.GameVersionID); versionErr == nil {
		gameVersion = version.Name
		if gameVersion == "" {
			gameVersion = version.ID
		}
	}
	mods, err := s.ListMods(ctx, instanceID)
	if err != nil {
		return modpack.Report{}, err
	}
	build := modpack.Build{
		GameVersion: gameVersion,
		Mods:        make([]modpack.ModInstall, 0, len(mods)),
	}
	for _, mod := range mods {
		install := modpack.ModInstall{
			Name:     mod.Name,
			Version:  mod.Version,
			FileName: mod.FileName,
			Enabled:  mod.Enabled,
		}
		if modID, _, ok := parseModDBSource(mod.Source); ok {
			install.ModID = modID
			install.Managed = true
		}
		if info, infoErr := readModArchiveInfo(mod.FilePath); infoErr == nil {
			for dependencyID := range info.Dependencies {
				install.Dependencies = append(install.Dependencies, dependencyID)
			}
		}
		build.Mods = append(build.Mods, install)
	}
	return modpack.Analyze(ctx, build, modpackCatalogAdapter{catalog: s.modCatalog})
}
