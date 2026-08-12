package mods

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/waxlight/waxlight-launcher/internal/errs"
)

// LinkLocalMods recognizes locally installed mods that are not yet managed by
// the launcher and binds them to their catalog entries. Each bound mod gains a
// DownloadedMod record and is marked as managed so updates and catalog state
// apply exactly like for mods downloaded through the launcher.
func (service *CatalogService) LinkLocalMods(ctx context.Context, instanceID string) (LinkLocalModsResult, error) {
	result := LinkLocalModsResult{}
	if err := service.gate.Begin(); err != nil {
		return result, err
	}
	defer service.gate.End()
	instance, err := service.repository.GetInstance(ctx, instanceID)
	if err != nil {
		return result, err
	}
	instanceRelease, err := service.lockInstanceMutations(instanceID)
	if err != nil {
		return result, err
	}
	defer instanceRelease()
	mods, err := service.lister.ListMods(ctx, instanceID)
	if err != nil {
		return result, err
	}
	for _, mod := range mods {
		if mod.Managed || !IsLocalModSource(mod.Source) {
			continue
		}
		downloaded, link, err := service.linkLocalModFile(ctx, mod.FilePath, false, mod.Name)
		if err != nil && !errors.Is(err, errLocalModAlreadyExists) {
			switch {
			case errors.Is(err, errLocalModNotMatched):
				result.NotMatched = append(result.NotMatched, link)
			default:
				result.Failed = append(result.Failed, link)
			}
			continue
		}
		updated := mod
		updated.Source = ModDBSource(downloaded.ModID, downloaded.VersionID)
		updated.Managed = true
		updated.Version = downloaded.DownloadedVersion
		updated.UpdatedAt = service.now().UTC()
		if err := service.repository.SaveMod(ctx, updated); err != nil {
			link.Reason = "Could not save the linked mod metadata"
			result.Failed = append(result.Failed, link)
			continue
		}
		service.events.Publish("mod:linked", updated)
		result.Linked = append(result.Linked, link)
	}
	slog.Info("local mods linked", "instance", instance.Name, "linked", len(result.Linked), "notMatched", len(result.NotMatched))
	return result, nil
}

// UploadMods imports local mod files into the launcher library, binding each to
// its catalog entry so it behaves like a mod downloaded through the launcher.
func (service *CatalogService) UploadMods(ctx context.Context, sourcePaths []string) (UploadModsResult, error) {
	result := UploadModsResult{}
	if err := service.gate.Begin(); err != nil {
		return result, err
	}
	defer service.gate.End()
	if len(sourcePaths) == 0 {
		return result, errs.NewError(errs.ErrValidation, "Select at least one mod file")
	}
	for _, sourcePath := range sourcePaths {
		downloaded, link, err := service.linkLocalModFile(ctx, sourcePath, true, "")
		switch {
		case err == nil:
			result.Linked = append(result.Linked, link)
			service.bindMatchingInstanceMods(ctx, downloaded)
		case errors.Is(err, errLocalModNotMatched):
			result.NotMatched = append(result.NotMatched, link)
		case errors.Is(err, errLocalModAlreadyExists):
			result.Skipped = append(result.Skipped, filepath.Base(sourcePath))
		default:
			result.Failed = append(result.Failed, link)
		}
	}
	if len(result.Linked) == 0 && len(result.Failed) > 0 {
		return result, errs.NewError(ErrInvalidModFile, "No mods were added to the library")
	}
	return result, nil
}

// bindMatchingInstanceMods binds local installed mods in any instance that
// correspond to the given catalog record, so the launcher recognizes that the
// mod is already installed there.
func (service *CatalogService) bindMatchingInstanceMods(ctx context.Context, target DownloadedMod) {
	instances, err := service.repository.ListInstances(ctx)
	if err != nil {
		slog.Warn("could not bind local mods to catalog entries", "error", err)
		return
	}
	for _, instance := range instances {
		instanceRelease, lockErr := service.lockInstanceMutations(instance.ID)
		if lockErr != nil {
			slog.Debug("skipping catalog metadata binding for a busy instance", "instance", instance.Name)
			continue
		}
		mods, err := service.repository.ListMods(ctx, instance.ID)
		if err != nil {
			instanceRelease()
			slog.Warn("could not list mods while binding catalog entries", "instance", instance.Name, "error", err)
			continue
		}
		for _, mod := range mods {
			if mod.Managed || !IsLocalModSource(mod.Source) || mod.FilePath == "" {
				continue
			}
			match, found := matchLocalModForFile(ctx, service.catalog, mod.FilePath)
			if !found || match.details.ID != target.ModID || match.version.ID != target.VersionID {
				continue
			}
			updated := mod
			updated.Source = ModDBSource(match.details.ID, match.version.ID)
			updated.Managed = true
			updated.Version = match.version.Version
			updated.UpdatedAt = service.now().UTC()
			if err := service.repository.SaveMod(ctx, updated); err == nil {
				service.events.Publish("mod:linked", updated)
			}
		}
		instanceRelease()
	}
}

// linkLocalModFile matches a local mod file against the catalog and persists a
// DownloadedMod record for it. When copyIntoCache is true the file is copied
// into the mod cache; otherwise the record references the file in place.
func (service *CatalogService) linkLocalModFile(
	ctx context.Context,
	sourcePath string,
	copyIntoCache bool,
	discoveredName string,
) (DownloadedMod, LocalModLink, error) {
	link := LocalModLink{Path: sourcePath, FileName: filepath.Base(sourcePath)}

	extension := strings.ToLower(filepath.Ext(sourcePath))
	if extension != ".zip" && extension != ".cs" && extension != ".dll" {
		link.Reason = "Unsupported mod file type"
		return DownloadedMod{}, link, errs.NewError(ErrInvalidModFile, "Unsupported mod file type")
	}
	info, err := ReadModArchiveInfo(sourcePath)
	if err != nil {
		return DownloadedMod{}, link, err
	}
	link.Name = discoveredName
	if link.Name == "" {
		link.Name = strings.TrimSpace(info.ModID)
	}
	if link.Name == "" {
		link.Name = strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	}
	link.Version = strings.TrimSpace(info.Version)

	match, reason, found := matchLocalMod(ctx, service.catalog, info.ModID, info.Version, link.FileName)
	if !found {
		link.Reason = reason
		return DownloadedMod{}, link, errLocalModNotMatched
	}
	details := match.details
	version := match.version
	link.Name = details.Name
	link.Version = version.Version

	if existing, getErr := service.downloads.Get(ctx, details.ID, version.ID); getErr == nil {
		if info, statErr := os.Stat(existing.FilePath); statErr == nil && !info.IsDir() {
			link.ModID = existing.ModID
			link.VersionID = existing.VersionID
			link.Slug = existing.Slug
			link.LatestVersion = existing.LatestVersion
			link.UpdateAvailable = existing.UpdateAvailable
			return existing, link, errLocalModAlreadyExists
		}
	}

	destination := sourcePath
	if copyIntoCache {
		path, err := service.cacheModDestination(details, version, link.FileName)
		if err != nil {
			link.Reason = err.Error()
			return DownloadedMod{}, link, err
		}
		if info, statErr := os.Stat(path); statErr != nil || info.IsDir() {
			if err := copyModFile(ctx, sourcePath, path); err != nil {
				link.Reason = "Could not copy the mod into the library"
				return DownloadedMod{}, link, err
			}
		}
		destination = path
	}

	size, err := os.Stat(destination)
	if err != nil {
		link.Reason = "Could not read the mod file"
		return DownloadedMod{}, link, err
	}
	downloaded := DownloadedMod{
		SchemaVersion:     1,
		ModID:             details.ID,
		Slug:              details.Slug,
		Name:              details.Name,
		AuthorName:        details.AuthorName,
		ImageURL:          details.ImageURL,
		Side:              details.Side,
		VersionID:         version.ID,
		DownloadedVersion: version.Version,
		GameVersions:      append([]string(nil), version.GameVersions...),
		FileName:          filepath.Base(destination),
		FilePath:          destination,
		FileSize:          size.Size(),
		Checksum:          version.Checksum,
		DownloadURL:       version.DownloadURL,
		DownloadedAt:      service.now().UTC(),
		LatestVersion:     details.LatestVersion,
		UpdateAvailable:   details.LatestVersion != "" && details.LatestVersion != version.Version,
	}
	if err := service.downloads.Save(ctx, downloaded); err != nil {
		link.Reason = "Could not save the mod metadata"
		return DownloadedMod{}, link, err
	}
	link.ModID = downloaded.ModID
	link.VersionID = downloaded.VersionID
	link.Slug = downloaded.Slug
	link.LatestVersion = downloaded.LatestVersion
	link.UpdateAvailable = downloaded.UpdateAvailable
	service.tasks.EmitDownloadsChanged("", downloaded.ModID, []modDownloadedDependencyEvent{})
	service.reportEvent(ctx, EventModDownloaded)
	return downloaded, link, nil
}

// cacheModDestination resolves the library destination of a local mod file,
// preferring the catalog file name.
func (service *CatalogService) cacheModDestination(
	details ModDetails,
	version ModVersion,
	localBase string,
) (string, error) {
	for _, candidate := range []string{version.FileName, localBase} {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		path, err := service.downloads.FilePath(details.ID, version.ID, candidate)
		if err == nil {
			return path, nil
		}
	}
	return "", errs.NewError(ErrInvalidModFile, "Unsupported mod file type")
}
