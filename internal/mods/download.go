package mods

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/downloads"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/errs"
)

// modInstallPlanItem is one release of the resolved install plan for a
// catalog download.
type modInstallPlanItem struct {
	Downloaded    DownloadedMod
	Root          bool
	DownloadedNow bool
}

// resolveAndDownloadCatalogMod downloads a catalog mod, resolves its
// dependencies recursively, and appends every release to the install plan.
func (service *CatalogService) resolveAndDownloadCatalogMod(
	ctx context.Context,
	taskID string,
	details ModDetails,
	selected ModVersion,
	gameVersions []string,
	allowIncompatible bool,
	root bool,
	resolved map[string]ModVersion,
	visiting map[string]struct{},
	plan *[]modInstallPlanItem,
	newDownloads *[]DownloadedMod,
) (DownloadedMod, error) {
	canonicalID := canonicalCatalogModID(details)
	if canonicalID == "" {
		return DownloadedMod{}, errs.NewError(ErrModCatalog, "The catalog returned a mod without an ID")
	}
	if resolvedVersion, ok := resolved[canonicalID]; ok {
		return service.downloads.Get(ctx, details.ID, resolvedVersion.ID)
	}
	if _, cycle := visiting[canonicalID]; cycle {
		return DownloadedMod{}, errs.NewError(
			ErrModCatalog,
			fmt.Sprintf("Circular dependency detected while resolving %s", details.Name),
		)
	}
	visiting[canonicalID] = struct{}{}
	defer delete(visiting, canonicalID)

	downloaded, downloadedNow, err := service.downloadCatalogVersion(ctx, taskID, details, selected)
	if err != nil {
		return DownloadedMod{}, err
	}
	if downloadedNow {
		*newDownloads = append(*newDownloads, downloaded)
	}

	archiveInfo, err := ReadModArchiveInfo(downloaded.FilePath)
	if err != nil {
		return DownloadedMod{}, err
	}
	dependencyIDs := make([]string, 0, len(archiveInfo.Dependencies))
	for dependencyID := range archiveInfo.Dependencies {
		if !isBuiltInModDependency(dependencyID) {
			dependencyIDs = append(dependencyIDs, dependencyID)
		}
	}
	sort.Strings(dependencyIDs)

	for _, dependencyID := range dependencyIDs {
		requirement := strings.TrimSpace(archiveInfo.Dependencies[dependencyID])
		service.tasks.EmitProgress(
			taskID,
			details.ID,
			phaseResolving,
			0,
			0,
			0,
			fmt.Sprintf("Resolving dependency %s for %s", dependencyID, details.Name),
		)

		dependencyDetails, getErr := service.catalog.Get(ctx, dependencyID)
		if getErr != nil {
			return DownloadedMod{}, &errs.AppError{
				Code:    ErrModCatalog,
				Message: fmt.Sprintf("Could not resolve required dependency %s for %s", dependencyID, details.Name),
				Cause:   getErr,
			}
		}
		dependencyVersion, found := findDependencyVersion(
			dependencyDetails.Versions,
			requirement,
			gameVersions,
			allowIncompatible,
		)
		if !found {
			versionText := requirement
			if versionText == "" || versionText == "*" {
				versionText = "any version"
			}
			return DownloadedMod{}, errs.NewError(
				ErrModVersionNotFound,
				fmt.Sprintf(
					"No compatible release of dependency %s (%s) was found for %s",
					dependencyDetails.Name,
					versionText,
					details.Name,
				),
			)
		}

		dependencyCanonicalID := canonicalCatalogModID(dependencyDetails)
		if alreadyResolved, ok := resolved[dependencyCanonicalID]; ok {
			if modVersionSatisfies(alreadyResolved.Version, requirement) {
				continue
			}
			// A later branch can require a newer version of a shared library.
			// Replace the earlier plan item with the version satisfying this branch.
			delete(resolved, dependencyCanonicalID)
			removeCatalogModFromInstallPlan(plan, dependencyDetails.ID)
		}

		if _, resolveErr := service.resolveAndDownloadCatalogMod(
			ctx,
			taskID,
			dependencyDetails,
			dependencyVersion,
			gameVersions,
			allowIncompatible,
			false,
			resolved,
			visiting,
			plan,
			newDownloads,
		); resolveErr != nil {
			return DownloadedMod{}, resolveErr
		}
	}

	resolved[canonicalID] = selected
	*plan = append(*plan, modInstallPlanItem{
		Downloaded:    downloaded,
		Root:          root,
		DownloadedNow: downloadedNow,
	})
	return downloaded, nil
}

// downloadCatalogVersion downloads a catalog release into the library. A
// cached artifact of the release is reused when present; concurrent downloads
// of the same release are rejected with a mod-already-active error.
func (service *CatalogService) downloadCatalogVersion(
	ctx context.Context,
	taskID string,
	details ModDetails,
	selected ModVersion,
) (DownloadedMod, bool, error) {
	if !strings.HasPrefix(selected.DownloadURL, "https://") {
		return DownloadedMod{}, false, errs.NewError(
			ErrInvalidModFile,
			fmt.Sprintf("The catalog returned an unsafe download URL for %s", details.Name),
		)
	}

	destination, err := service.downloads.FilePath(details.ID, selected.ID, selected.FileName)
	if err != nil {
		return DownloadedMod{}, false, err
	}
	downloaded, getErr := service.downloads.Get(ctx, details.ID, selected.ID)
	if getErr == nil {
		if info, statErr := os.Stat(downloaded.FilePath); statErr == nil && !info.IsDir() {
			return downloaded, false, nil
		}
	}

	if err := service.tasks.Claim(taskID, details.ID, selected.ID); err != nil {
		var busy *ReleaseBusyError
		if errors.As(err, &busy) {
			return DownloadedMod{}, false, &errs.AppError{
				Code:    ErrModAlreadyActive,
				Message: fmt.Sprintf("%s is already downloading", details.Name),
				Cause:   busy,
			}
		}
		return DownloadedMod{}, false, err
	}

	// Another task may have completed the same dependency before this task
	// acquired the reservation.
	if cached, cacheErr := service.downloads.Get(ctx, details.ID, selected.ID); cacheErr == nil {
		if info, statErr := os.Stat(cached.FilePath); statErr == nil && !info.IsDir() {
			return cached, false, nil
		}
	}

	service.tasks.EmitProgress(taskID, details.ID, phasePreparing, 0, 0, 0, messagePreparing)
	progress := make(chan downloads.Progress, 16)
	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		for update := range progress {
			percent := float64(0)
			if update.TotalBytes > 0 {
				percent = float64(update.DownloadedBytes) / float64(update.TotalBytes)
			}
			service.tasks.EmitProgress(
				taskID,
				details.ID,
				phaseDownloading,
				update.DownloadedBytes,
				update.TotalBytes,
				percent,
				"Downloading "+details.Name,
			)
		}
	}()

	err = service.downloader.Download(ctx, downloads.Request{
		URL:               selected.DownloadURL,
		DestinationPath:   destination,
		ExpectedChecksum:  selected.Checksum,
		ChecksumAlgorithm: "sha256",
		Resume:            true,
		MaxBytes:          maxModDownloadBytes,
	}, progress)
	close(progress)
	<-progressDone
	if err != nil {
		if removeErr := os.Remove(destination + ".partial"); removeErr != nil {
			slog.Debug("could not remove the partial download", "path", destination, "error", removeErr)
		}
		message := "Could not download the mod"
		if errors.Is(err, context.Canceled) {
			message = "Download cancelled"
		} else {
			service.reportModDownloadError(err)
		}
		return DownloadedMod{}, false, &errs.AppError{
			Code:      errs.ErrDownloadFailed,
			Message:   message,
			Retryable: true,
			Cause:     err,
		}
	}

	info, statErr := os.Stat(destination)
	if statErr != nil {
		return DownloadedMod{}, false, statErr
	}
	downloaded = DownloadedMod{
		SchemaVersion:     1,
		ModID:             details.ID,
		Slug:              details.Slug,
		Name:              details.Name,
		AuthorName:        details.AuthorName,
		ImageURL:          details.ImageURL,
		Side:              details.Side,
		VersionID:         selected.ID,
		DownloadedVersion: selected.Version,
		GameVersions:      append([]string(nil), selected.GameVersions...),
		Tags:              append([]string{}, details.Tags...),
		FileName:          filepath.Base(destination),
		FilePath:          destination,
		FileSize:          info.Size(),
		Checksum:          selected.Checksum,
		DownloadURL:       selected.DownloadURL,
		DownloadedAt:      service.now().UTC(),
		LatestVersion:     details.LatestVersion,
	}
	if err := service.downloads.Save(ctx, downloaded); err != nil {
		return DownloadedMod{}, false, err
	}
	service.reportEvent(ctx, EventModDownloaded)
	return downloaded, true, nil
}

// DownloadRelease fetches the exact catalog release identified by versionID
// for the given mod. An already cached artifact of that release is reused when
// present — including offline — otherwise the shared downloader fetches it and
// verifies its checksum. The release identity (versionID) is authoritative: a
// newer release is never substituted.
func (service *CatalogService) DownloadRelease(
	ctx context.Context,
	modID string,
	versionID string,
) (DownloadedMod, error) {
	if cached, cacheErr := service.downloads.Get(ctx, modID, versionID); cacheErr == nil {
		if info, statErr := os.Stat(cached.FilePath); statErr == nil && !info.IsDir() {
			return cached, nil
		}
	}
	details, err := service.catalog.Get(ctx, modID)
	if err != nil {
		return DownloadedMod{}, err
	}
	selected, ok := FindModVersion(details.Versions, versionID)
	if !ok {
		return DownloadedMod{}, errs.NewError(
			ErrModVersionNotFound,
			"The exact mod release is no longer available",
		)
	}
	taskID := service.newID()
	downloadCtx, _, err := service.tasks.Begin(ctx, taskID, modID, versionID)
	if err != nil {
		return DownloadedMod{}, err
	}
	defer service.tasks.Release(taskID)
	downloaded, _, err := service.downloadCatalogVersion(
		downloadCtx,
		taskID,
		details,
		selected,
	)
	return downloaded, err
}

// installModPlan installs every release of the plan into one instance and
// reports the first failure.
func (service *CatalogService) installModPlan(
	ctx context.Context,
	plan []modInstallPlanItem,
	instance InstanceRef,
) ModInstallationResult {
	result := ModInstallationResult{
		InstanceID:   instance.ID,
		InstanceName: instance.Name,
	}
	for _, item := range plan {
		installation := service.installDownloadedMod(ctx, item.Downloaded, instance)
		if !installation.Installed {
			if item.Root {
				return installation
			}
			result.Message = fmt.Sprintf(
				"Could not install dependency %s: %s",
				item.Downloaded.Name,
				installation.Message,
			)
			return result
		}
		if item.Root {
			return installation
		}
	}
	result.Message = "The installation plan did not contain the requested mod"
	return result
}

// installDownloadedMod installs a cached catalog release into an instance,
// replacing an older release of the same mod.
func (service *CatalogService) installDownloadedMod(
	ctx context.Context,
	downloaded DownloadedMod,
	instance InstanceRef,
) ModInstallationResult {
	result := ModInstallationResult{InstanceID: instance.ID, InstanceName: instance.Name}
	mods, err := service.repository.ListMods(ctx, instance.ID)
	if err != nil {
		result.Message = "Could not inspect installed mods"
		return result
	}
	sourcePrefix := "moddb:" + downloaded.ModID + ":"
	var previous *InstalledMod
	for index := range mods {
		if strings.HasPrefix(mods[index].Source, sourcePrefix) {
			copy := mods[index]
			previous = &copy
			if copy.Source == ModDBSource(downloaded.ModID, downloaded.VersionID) && copy.Version == downloaded.DownloadedVersion {
				result.Installed = true
				result.Message = "Already installed"
				return result
			}
			break
		}
	}
	oldPath := ""
	if previous != nil {
		oldPath = previous.FilePath
	}
	path, size, err := service.files.InstallOrReplace(ctx, downloaded.FilePath, instance.Directory, oldPath)
	if err != nil {
		result.Message = friendlyInstallError(err)
		return result
	}
	if previous != nil {
		if err := service.repository.DeleteMod(ctx, previous.ID); err != nil {
			result.Message = "Installed the file but could not update its metadata"
			return result
		}
		if modID, versionID, ok := ParseModDBSource(previous.Source); ok {
			service.removeSupersededCacheVersion(ctx, modID, versionID)
		}
	}
	now := service.now().UTC()
	installed := InstalledMod{
		ID:          service.newID(),
		InstanceID:  instance.ID,
		Name:        downloaded.Name,
		Version:     downloaded.DownloadedVersion,
		FileName:    filepath.Base(path),
		FilePath:    path,
		Enabled:     true,
		Managed:     true,
		Source:      ModDBSource(downloaded.ModID, downloaded.VersionID),
		SizeBytes:   size,
		InstalledAt: now,
		UpdatedAt:   now,
	}
	if err := service.repository.SaveMod(ctx, installed); err != nil {
		result.Message = "Installed the file but could not save its metadata"
		return result
	}
	result.Installed = true
	result.Message = "Installed"
	service.events.Publish("mod:installed", installed)
	return result
}
