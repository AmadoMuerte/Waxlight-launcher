package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
)

const maxModDownloadBytes int64 = 512 << 20

func (s *Service) SearchMods(
	ctx context.Context,
	query domain.ModSearchQuery,
) (domain.ModSearchResult, error) {
	if s.modCatalog == nil {
		return domain.ModSearchResult{}, domain.NewError(domain.ErrModCatalog, "The mod catalog is not configured")
	}
	result, err := s.modCatalog.Search(ctx, query)
	if err != nil {
		return result, err
	}
	downloaded, _ := s.ListDownloadedMods(ctx)
	byID := make(map[string]domain.DownloadedMod, len(downloaded))
	for _, item := range downloaded {
		current, exists := byID[item.ModID]
		if !exists || item.DownloadedAt.After(current.DownloadedAt) {
			byID[item.ModID] = item
		}
	}
	for index := range result.Items {
		if local, ok := byID[result.Items[index].ID]; ok {
			result.Items[index].IsDownloaded = true
			result.Items[index].IsInstalled = len(local.InstalledInstances) > 0
			result.Items[index].UpdateAvailable = local.UpdateAvailable
		}
	}
	return result, nil
}

func (s *Service) GetCatalogMod(
	ctx context.Context,
	modID string,
) (domain.ModDetails, error) {
	if s.modCatalog == nil {
		return domain.ModDetails{}, domain.NewError(domain.ErrModCatalog, "The mod catalog is not configured")
	}
	details, err := s.modCatalog.Get(ctx, modID)
	if err != nil {
		return details, err
	}
	downloaded, _ := s.ListDownloadedMods(ctx)
	for _, local := range downloaded {
		if local.ModID == details.ID {
			details.IsDownloaded = true
			details.IsInstalled = details.IsInstalled || len(local.InstalledInstances) > 0
			details.UpdateAvailable = details.UpdateAvailable || local.UpdateAvailable
		}
	}
	return details, nil
}

func (s *Service) ListDownloadedMods(ctx context.Context) ([]domain.DownloadedMod, error) {
	if s.modDownloads == nil {
		return []domain.DownloadedMod{}, nil
	}
	items, err := s.modDownloads.List(ctx)
	if err != nil {
		return nil, err
	}
	instances, err := s.store.ListInstances(ctx)
	if err != nil {
		return nil, err
	}
	instanceNames := make(map[string]string, len(instances))
	installedBySource := make(map[string][]domain.InstalledModInstance)
	for _, instance := range instances {
		instanceNames[instance.ID] = instance.Name
		mods, listErr := s.store.ListMods(ctx, instance.ID)
		if listErr != nil {
			continue
		}
		for _, installed := range mods {
			modID, versionID, ok := parseModDBSource(installed.Source)
			if !ok {
				continue
			}
			key := modDownloadKey(modID, versionID)
			installedBySource[key] = append(installedBySource[key], domain.InstalledModInstance{
				InstanceID:   instance.ID,
				InstanceName: instance.Name,
				Version:      installed.Version,
				Enabled:      installed.Enabled,
			})
		}
	}
	for index := range items {
		items[index].InstalledInstances = installedBySource[modDownloadKey(items[index].ModID, items[index].VersionID)]
		if s.modCatalog != nil {
			if details, detailErr := s.modCatalog.Get(ctx, items[index].ModID); detailErr == nil {
				items[index].LatestVersion = details.LatestVersion
				items[index].UpdateAvailable = details.LatestVersion != "" && details.LatestVersion != items[index].DownloadedVersion
			}
		}
	}
	sort.Slice(items, func(left, right int) bool {
		return items[left].DownloadedAt.After(items[right].DownloadedAt)
	})
	_ = instanceNames
	return items, nil
}

func (s *Service) DownloadCatalogMod(
	ctx context.Context,
	request domain.DownloadModRequest,
) (domain.ModInstallResult, error) {
	if s.modCatalog == nil || s.modDownloads == nil || s.downloader == nil {
		return domain.ModInstallResult{}, domain.NewError(domain.ErrModCatalog, "Mod downloads are not configured")
	}
	details, err := s.modCatalog.Get(ctx, request.ModID)
	if err != nil {
		return domain.ModInstallResult{}, err
	}
	selected, ok := findModVersion(details.Versions, request.VersionID)
	if !ok {
		return domain.ModInstallResult{}, domain.NewError(domain.ErrModVersionNotFound, "Mod version not found")
	}
	if !strings.HasPrefix(selected.DownloadURL, "https://") {
		return domain.ModInstallResult{}, domain.NewError(domain.ErrInvalidModFile, "The catalog returned an unsafe download URL")
	}

	instances := make([]domain.Instance, 0, len(request.InstanceIDs))
	seenInstances := make(map[string]struct{}, len(request.InstanceIDs))
	for _, instanceID := range request.InstanceIDs {
		if _, duplicate := seenInstances[instanceID]; duplicate {
			continue
		}
		seenInstances[instanceID] = struct{}{}
		instance, getErr := s.store.GetInstance(ctx, instanceID)
		if getErr != nil {
			return domain.ModInstallResult{}, getErr
		}
		version, versionErr := s.store.GetVersion(ctx, instance.GameVersionID)
		if versionErr != nil {
			return domain.ModInstallResult{}, versionErr
		}
		gameVersion := version.Name
		if gameVersion == "" {
			gameVersion = version.ID
		}
		if !request.AllowIncompatible && !modSupportsVersion(selected.GameVersions, gameVersion) {
			return domain.ModInstallResult{}, domain.NewError(
				domain.ErrModIncompatible,
				fmt.Sprintf("%s does not list support for Vintage Story %s", details.Name, gameVersion),
			)
		}
		instances = append(instances, instance)
	}

	key := modDownloadKey(details.ID, selected.ID)
	taskID := newID()
	downloadCtx, cancel := context.WithCancel(ctx)
	s.operationsMu.Lock()
	if existingTask, active := s.activeModDownloads[key]; active {
		s.operationsMu.Unlock()
		cancel()
		return domain.ModInstallResult{TaskID: existingTask}, domain.NewError(domain.ErrModAlreadyActive, "This mod is already downloading")
	}
	s.activeModDownloads[key] = taskID
	s.operationCancels[taskID] = cancel
	s.operationsMu.Unlock()
	defer func() {
		cancel()
		s.operationsMu.Lock()
		delete(s.activeModDownloads, key)
		delete(s.operationCancels, taskID)
		s.operationsMu.Unlock()
	}()

	destination, err := s.modDownloads.FilePath(details.ID, selected.ID, selected.FileName)
	if err != nil {
		return domain.ModInstallResult{TaskID: taskID}, err
	}
	downloaded, getErr := s.modDownloads.Get(downloadCtx, details.ID, selected.ID)
	needsDownload := getErr != nil
	if !needsDownload {
		if info, statErr := os.Stat(downloaded.FilePath); statErr != nil || info.IsDir() {
			needsDownload = true
		}
	}
	if needsDownload {
		s.emitModProgress(taskID, details.ID, "preparing", 0, 0, 0, "Preparing download")
		progress := make(chan DownloadProgress, 16)
		progressDone := make(chan struct{})
		go func() {
			defer close(progressDone)
			for update := range progress {
				percent := float64(0)
				if update.TotalBytes > 0 {
					percent = float64(update.DownloadedBytes) / float64(update.TotalBytes)
				}
				s.emitModProgress(taskID, details.ID, "downloading", update.DownloadedBytes, update.TotalBytes, percent, "Downloading "+details.Name)
			}
		}()
		err = s.downloader.Download(downloadCtx, DownloadRequest{
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
			_ = os.Remove(destination + ".partial")
			phase := "failed"
			message := "Could not download the mod"
			if errors.Is(err, context.Canceled) {
				message = "Download cancelled"
			}
			s.emitModProgress(taskID, details.ID, phase, 0, 0, 0, message)
			return domain.ModInstallResult{TaskID: taskID}, &domain.AppError{Code: domain.ErrDownloadFailed, Message: message, Retryable: true, Cause: err}
		}
		info, statErr := os.Stat(destination)
		if statErr != nil {
			return domain.ModInstallResult{TaskID: taskID}, statErr
		}
		downloaded = domain.DownloadedMod{
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
			FileName:          filepath.Base(destination),
			FilePath:          destination,
			FileSize:          info.Size(),
			Checksum:          selected.Checksum,
			DownloadURL:       selected.DownloadURL,
			DownloadedAt:      time.Now().UTC(),
			LatestVersion:     details.LatestVersion,
		}
		if err := s.modDownloads.Save(downloadCtx, downloaded); err != nil {
			return domain.ModInstallResult{TaskID: taskID}, err
		}
	}

	result := domain.ModInstallResult{TaskID: taskID, Downloaded: downloaded}
	if request.DownloadOnly || len(instances) == 0 {
		s.emitModProgress(taskID, details.ID, "complete", downloaded.FileSize, downloaded.FileSize, 1, "Download complete")
		return result, nil
	}
	for _, instance := range instances {
		installation := s.installDownloadedMod(downloadCtx, downloaded, instance)
		result.Installations = append(result.Installations, installation)
	}
	s.emitModProgress(taskID, details.ID, "complete", downloaded.FileSize, downloaded.FileSize, 1, "Installation complete")
	return result, nil
}

func (s *Service) InstallDownloadedMod(
	ctx context.Context,
	modID string,
	versionID string,
	instanceIDs []string,
	allowIncompatible bool,
) (domain.ModInstallResult, error) {
	return s.DownloadCatalogMod(ctx, domain.DownloadModRequest{
		ModID:             modID,
		VersionID:         versionID,
		InstanceIDs:       instanceIDs,
		AllowIncompatible: allowIncompatible,
	})
}

func (s *Service) RemoveDownloadedMod(ctx context.Context, modID, versionID string) error {
	if s.modDownloads == nil {
		return domain.NewError(domain.ErrModVersionNotFound, "Downloaded mod version not found")
	}
	return s.modDownloads.Delete(ctx, modID, versionID)
}

func (s *Service) CancelModTask(taskID string) error {
	s.operationsMu.Lock()
	cancel, ok := s.operationCancels[taskID]
	s.operationsMu.Unlock()
	if !ok {
		return domain.NewError(domain.ErrOperationNotFound, "Mod task not found")
	}
	cancel()
	return nil
}

func (s *Service) installDownloadedMod(
	ctx context.Context,
	downloaded domain.DownloadedMod,
	instance domain.Instance,
) domain.ModInstallationResult {
	result := domain.ModInstallationResult{InstanceID: instance.ID, InstanceName: instance.Name}
	mods, err := s.store.ListMods(ctx, instance.ID)
	if err != nil {
		result.Message = "Could not inspect installed mods"
		return result
	}
	sourcePrefix := "moddb:" + downloaded.ModID + ":"
	var previous *domain.InstalledMod
	for index := range mods {
		if strings.HasPrefix(mods[index].Source, sourcePrefix) {
			copy := mods[index]
			previous = &copy
			if copy.Source == modDBSource(downloaded.ModID, downloaded.VersionID) && copy.Version == downloaded.DownloadedVersion {
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
	path, size, err := s.modFiles.InstallOrReplace(ctx, downloaded.FilePath, instance.Directory, oldPath)
	if err != nil {
		result.Message = friendlyInstallError(err)
		return result
	}
	if previous != nil {
		if err := s.store.DeleteMod(ctx, previous.ID); err != nil {
			result.Message = "Installed the file but could not update its metadata"
			return result
		}
	}
	now := time.Now().UTC()
	installed := domain.InstalledMod{
		ID:          newID(),
		InstanceID:  instance.ID,
		Name:        downloaded.Name,
		Version:     downloaded.DownloadedVersion,
		FileName:    filepath.Base(path),
		FilePath:    path,
		Enabled:     true,
		Managed:     true,
		Source:      modDBSource(downloaded.ModID, downloaded.VersionID),
		SizeBytes:   size,
		InstalledAt: now,
		UpdatedAt:   now,
	}
	if err := s.store.SaveMod(ctx, installed); err != nil {
		result.Message = "Installed the file but could not save its metadata"
		return result
	}
	result.Installed = true
	result.Message = "Installed"
	s.emit("mod:installed", installed)
	return result
}

func (s *Service) emitModProgress(
	taskID, modID, phase string,
	downloadedBytes, totalBytes int64,
	progress float64,
	message string,
) {
	s.emit("mods:task-progress", map[string]any{
		"taskId": taskID, "modId": modID, "phase": phase,
		"downloadedBytes": downloadedBytes, "totalBytes": totalBytes,
		"progress": progress, "message": message,
	})
}

func findModVersion(versions []domain.ModVersion, id string) (domain.ModVersion, bool) {
	if id == "" && len(versions) > 0 {
		for _, version := range versions {
			if version.ReleaseType == "stable" {
				return version, true
			}
		}
		return versions[0], true
	}
	for _, version := range versions {
		if version.ID == id {
			return version, true
		}
	}
	return domain.ModVersion{}, false
}

func modSupportsVersion(versions []string, requested string) bool {
	for _, version := range versions {
		if version == requested {
			return true
		}
		majorMinor := strings.Join(strings.Split(version, ".")[:min(2, len(strings.Split(version, ".")))], ".")
		if majorMinor != "" && strings.HasPrefix(requested, majorMinor+".") {
			return true
		}
	}
	return len(versions) == 0
}

func modDownloadKey(modID, versionID string) string { return modID + ":" + versionID }
func modDBSource(modID, versionID string) string    { return "moddb:" + modID + ":" + versionID }

func parseModDBSource(source string) (string, string, bool) {
	parts := strings.Split(source, ":")
	if len(parts) != 3 || parts[0] != "moddb" || parts[1] == "" || parts[2] == "" {
		return "", "", false
	}
	return parts[1], parts[2], true
}

func friendlyInstallError(err error) string {
	if errors.Is(err, os.ErrPermission) {
		return "Waxlight does not have permission to write to this instance"
	}
	return "Could not install the mod in this instance"
}
