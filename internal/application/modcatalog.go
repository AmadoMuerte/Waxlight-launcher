package application

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/waxlight/waxlight-launcher/internal/domain"
	"golang.org/x/mod/semver"
)

const (
	maxModDownloadBytes int64 = 512 << 20
	maxModInfoBytes     int64 = 1 << 20
)

type modArchiveInfo struct {
	ModID        string            `json:"modid"`
	Version      string            `json:"version"`
	Dependencies map[string]string `json:"dependencies"`
}

type modInstallPlanItem struct {
	Downloaded    domain.DownloadedMod
	Root          bool
	DownloadedNow bool
}

type modDownloadedDependencyEvent struct {
	ModID   string `json:"modId"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

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
	installedBySource := make(map[string][]domain.InstalledModInstance)
	for _, instance := range instances {
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
	}
	sort.Slice(items, func(left, right int) bool {
		return items[left].DownloadedAt.After(items[right].DownloadedAt)
	})
	return items, nil
}

func (s *Service) CheckModUpdates(
	ctx context.Context,
	modID string,
) ([]domain.DownloadedMod, error) {
	if s.modCatalog == nil || s.modDownloads == nil {
		return nil, domain.NewError(domain.ErrModCatalog, "The mod catalog is not configured")
	}
	details, err := s.modCatalog.Get(ctx, modID)
	if err != nil {
		return nil, err
	}
	items, err := s.modDownloads.List(ctx)
	if err != nil {
		return nil, err
	}
	for index := range items {
		if items[index].ModID != details.ID {
			continue
		}
		items[index].LatestVersion = details.LatestVersion
		items[index].UpdateAvailable = details.LatestVersion != "" &&
			details.LatestVersion != items[index].DownloadedVersion
		if err := s.modDownloads.Save(ctx, items[index]); err != nil {
			return nil, err
		}
	}
	return s.ListDownloadedMods(ctx)
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
	gameVersions := make([]string, 0, len(request.InstanceIDs))
	seenInstances := make(map[string]struct{}, len(request.InstanceIDs))
	seenGameVersions := make(map[string]struct{}, len(request.InstanceIDs))
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
		if _, seen := seenGameVersions[gameVersion]; !seen {
			seenGameVersions[gameVersion] = struct{}{}
			gameVersions = append(gameVersions, gameVersion)
		}
		instances = append(instances, instance)
	}

	key := modDownloadKey(details.ID, selected.ID)
	taskID := newID()
	downloadCtx, cancel := context.WithCancel(ctx)
	reservedKeys := map[string]struct{}{key: {}}

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
		for reservedKey := range reservedKeys {
			if s.activeModDownloads[reservedKey] == taskID {
				delete(s.activeModDownloads, reservedKey)
			}
		}
		delete(s.operationCancels, taskID)
		s.operationsMu.Unlock()
	}()

	plan := make([]modInstallPlanItem, 0, 4)
	resolved := make(map[string]domain.ModVersion)
	visiting := make(map[string]struct{})
	downloaded, err := s.resolveAndDownloadCatalogMod(
		downloadCtx,
		taskID,
		details,
		selected,
		gameVersions,
		request.AllowIncompatible,
		true,
		reservedKeys,
		resolved,
		visiting,
		&plan,
	)
	if err != nil {
		s.emitModProgress(taskID, details.ID, "failed", 0, 0, 0, dependencyFailureMessage(err))
		return domain.ModInstallResult{TaskID: taskID}, err
	}

	result := domain.ModInstallResult{TaskID: taskID, Downloaded: downloaded}
	defer s.emitModDownloadsChanged(taskID, details.ID, newlyDownloadedDependencies(plan))
	if request.DownloadOnly || len(instances) == 0 {
		s.emitModProgress(taskID, details.ID, "complete", downloaded.FileSize, downloaded.FileSize, 1, "Download complete")
		return result, nil
	}

	allInstalled := true
	for _, instance := range instances {
		installation := s.installModPlan(downloadCtx, plan, instance)
		result.Installations = append(result.Installations, installation)
		allInstalled = allInstalled && installation.Installed
	}
	if !allInstalled {
		s.emitModProgress(taskID, details.ID, "failed", downloaded.FileSize, downloaded.FileSize, 1, "Installation failed")
		return result, nil
	}
	s.emitModProgress(taskID, details.ID, "complete", downloaded.FileSize, downloaded.FileSize, 1, "Installation complete")
	return result, nil
}

func (s *Service) resolveAndDownloadCatalogMod(
	ctx context.Context,
	taskID string,
	details domain.ModDetails,
	selected domain.ModVersion,
	gameVersions []string,
	allowIncompatible bool,
	root bool,
	reservedKeys map[string]struct{},
	resolved map[string]domain.ModVersion,
	visiting map[string]struct{},
	plan *[]modInstallPlanItem,
) (domain.DownloadedMod, error) {
	canonicalID := canonicalCatalogModID(details)
	if canonicalID == "" {
		return domain.DownloadedMod{}, domain.NewError(domain.ErrModCatalog, "The catalog returned a mod without an ID")
	}
	if resolvedVersion, ok := resolved[canonicalID]; ok {
		return s.modDownloads.Get(ctx, details.ID, resolvedVersion.ID)
	}
	if _, cycle := visiting[canonicalID]; cycle {
		return domain.DownloadedMod{}, domain.NewError(
			domain.ErrModCatalog,
			fmt.Sprintf("Circular dependency detected while resolving %s", details.Name),
		)
	}
	visiting[canonicalID] = struct{}{}
	defer delete(visiting, canonicalID)

	downloaded, downloadedNow, err := s.downloadCatalogVersion(ctx, taskID, details, selected, reservedKeys)
	if err != nil {
		return domain.DownloadedMod{}, err
	}

	archiveInfo, err := readModArchiveInfo(downloaded.FilePath)
	if err != nil {
		return domain.DownloadedMod{}, err
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
		s.emitModProgress(
			taskID,
			details.ID,
			"resolving",
			0,
			0,
			0,
			fmt.Sprintf("Resolving dependency %s for %s", dependencyID, details.Name),
		)

		dependencyDetails, getErr := s.modCatalog.Get(ctx, dependencyID)
		if getErr != nil {
			return domain.DownloadedMod{}, &domain.AppError{
				Code:    domain.ErrModCatalog,
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
			return domain.DownloadedMod{}, domain.NewError(
				domain.ErrModVersionNotFound,
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
			if !modVersionSatisfies(alreadyResolved.Version, requirement) {
				return domain.DownloadedMod{}, domain.NewError(
					domain.ErrModVersionNotFound,
					fmt.Sprintf(
						"Dependency %s requires %s, but the resolved version is %s",
						dependencyDetails.Name,
						requirement,
						alreadyResolved.Version,
					),
				)
			}
			continue
		}

		if _, resolveErr := s.resolveAndDownloadCatalogMod(
			ctx,
			taskID,
			dependencyDetails,
			dependencyVersion,
			gameVersions,
			allowIncompatible,
			false,
			reservedKeys,
			resolved,
			visiting,
			plan,
		); resolveErr != nil {
			return domain.DownloadedMod{}, resolveErr
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

func (s *Service) downloadCatalogVersion(
	ctx context.Context,
	taskID string,
	details domain.ModDetails,
	selected domain.ModVersion,
	reservedKeys map[string]struct{},
) (domain.DownloadedMod, bool, error) {
	if !strings.HasPrefix(selected.DownloadURL, "https://") {
		return domain.DownloadedMod{}, false, domain.NewError(
			domain.ErrInvalidModFile,
			fmt.Sprintf("The catalog returned an unsafe download URL for %s", details.Name),
		)
	}

	destination, err := s.modDownloads.FilePath(details.ID, selected.ID, selected.FileName)
	if err != nil {
		return domain.DownloadedMod{}, false, err
	}
	downloaded, getErr := s.modDownloads.Get(ctx, details.ID, selected.ID)
	if getErr == nil {
		if info, statErr := os.Stat(downloaded.FilePath); statErr == nil && !info.IsDir() {
			return downloaded, false, nil
		}
	}

	key := modDownloadKey(details.ID, selected.ID)
	if _, owned := reservedKeys[key]; !owned {
		s.operationsMu.Lock()
		if existingTask, active := s.activeModDownloads[key]; active {
			s.operationsMu.Unlock()
			return domain.DownloadedMod{}, false, &domain.AppError{
				Code:    domain.ErrModAlreadyActive,
				Message: fmt.Sprintf("%s is already downloading", details.Name),
				Cause:   fmt.Errorf("task %s is downloading %s", existingTask, key),
			}
		}
		s.activeModDownloads[key] = taskID
		reservedKeys[key] = struct{}{}
		s.operationsMu.Unlock()

		// Another task may have completed the same dependency before this task
		// acquired the reservation.
		if cached, cacheErr := s.modDownloads.Get(ctx, details.ID, selected.ID); cacheErr == nil {
			if info, statErr := os.Stat(cached.FilePath); statErr == nil && !info.IsDir() {
				return cached, false, nil
			}
		}
	}

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
			s.emitModProgress(
				taskID,
				details.ID,
				"downloading",
				update.DownloadedBytes,
				update.TotalBytes,
				percent,
				"Downloading "+details.Name,
			)
		}
	}()

	err = s.downloader.Download(ctx, DownloadRequest{
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
		message := "Could not download the mod"
		if errors.Is(err, context.Canceled) {
			message = "Download cancelled"
		}
		return domain.DownloadedMod{}, false, &domain.AppError{
			Code:      domain.ErrDownloadFailed,
			Message:   message,
			Retryable: true,
			Cause:     err,
		}
	}

	info, statErr := os.Stat(destination)
	if statErr != nil {
		return domain.DownloadedMod{}, false, statErr
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
	if err := s.modDownloads.Save(ctx, downloaded); err != nil {
		return domain.DownloadedMod{}, false, err
	}
	return downloaded, true, nil
}

func (s *Service) installModPlan(
	ctx context.Context,
	plan []modInstallPlanItem,
	instance domain.Instance,
) domain.ModInstallationResult {
	result := domain.ModInstallationResult{
		InstanceID:   instance.ID,
		InstanceName: instance.Name,
	}
	for _, item := range plan {
		installation := s.installDownloadedMod(ctx, item.Downloaded, instance)
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

func readModArchiveInfo(filePath string) (modArchiveInfo, error) {
	reader, err := zip.OpenReader(filePath)
	if errors.Is(err, zip.ErrFormat) {
		// Keep supporting catalog entries that are not ZIP-based mods. Such
		// packages simply cannot advertise dependencies through modinfo.json.
		return modArchiveInfo{}, nil
	}
	if err != nil {
		return modArchiveInfo{}, &domain.AppError{
			Code:    domain.ErrInvalidModFile,
			Message: "Could not inspect the downloaded mod archive",
			Cause:   err,
		}
	}
	defer reader.Close()

	var modInfoFile *zip.File
	for _, file := range reader.File {
		name := strings.TrimPrefix(filepath.ToSlash(file.Name), "./")
		if strings.EqualFold(name, "modinfo.json") {
			modInfoFile = file
			break
		}
	}
	if modInfoFile == nil {
		return modArchiveInfo{}, nil
	}
	if modInfoFile.UncompressedSize64 > uint64(maxModInfoBytes) {
		return modArchiveInfo{}, domain.NewError(domain.ErrInvalidModFile, "modinfo.json is unexpectedly large")
	}

	file, err := modInfoFile.Open()
	if err != nil {
		return modArchiveInfo{}, &domain.AppError{
			Code:    domain.ErrInvalidModFile,
			Message: "Could not open modinfo.json",
			Cause:   err,
		}
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxModInfoBytes+1))
	if err != nil {
		return modArchiveInfo{}, &domain.AppError{
			Code:    domain.ErrInvalidModFile,
			Message: "Could not read modinfo.json",
			Cause:   err,
		}
	}
	if int64(len(data)) > maxModInfoBytes {
		return modArchiveInfo{}, domain.NewError(domain.ErrInvalidModFile, "modinfo.json is unexpectedly large")
	}

	var info modArchiveInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return modArchiveInfo{}, &domain.AppError{
			Code:    domain.ErrInvalidModFile,
			Message: "The downloaded mod contains an invalid modinfo.json",
			Cause:   err,
		}
	}
	if info.Dependencies == nil {
		info.Dependencies = map[string]string{}
	}
	return info, nil
}

func findDependencyVersion(
	versions []domain.ModVersion,
	requirement string,
	gameVersions []string,
	allowIncompatible bool,
) (domain.ModVersion, bool) {
	candidates := make([]domain.ModVersion, 0, len(versions))
	for _, version := range versions {
		if !strings.HasPrefix(version.DownloadURL, "https://") {
			continue
		}
		if !modVersionSatisfies(version.Version, requirement) {
			continue
		}
		if !allowIncompatible {
			compatible := true
			for _, gameVersion := range gameVersions {
				if !modSupportsVersion(version.GameVersions, gameVersion) {
					compatible = false
					break
				}
			}
			if !compatible {
				continue
			}
		}
		candidates = append(candidates, version)
	}
	if len(candidates) == 0 {
		return domain.ModVersion{}, false
	}

	sort.SliceStable(candidates, func(left, right int) bool {
		leftExact := modVersionExactlyMatches(candidates[left].Version, requirement)
		rightExact := modVersionExactlyMatches(candidates[right].Version, requirement)
		if leftExact != rightExact {
			return leftExact
		}

		leftStable := strings.EqualFold(candidates[left].ReleaseType, "stable")
		rightStable := strings.EqualFold(candidates[right].ReleaseType, "stable")
		if leftStable != rightStable {
			return leftStable
		}

		leftVersion := normalizeModSemver(candidates[left].Version)
		rightVersion := normalizeModSemver(candidates[right].Version)
		if leftVersion != "" && rightVersion != "" && leftVersion != rightVersion {
			return semver.Compare(leftVersion, rightVersion) > 0
		}
		if candidates[left].PublishedAt != nil && candidates[right].PublishedAt != nil &&
			!candidates[left].PublishedAt.Equal(*candidates[right].PublishedAt) {
			return candidates[left].PublishedAt.After(*candidates[right].PublishedAt)
		}
		return candidates[left].ID > candidates[right].ID
	})
	return candidates[0], true
}

func modVersionExactlyMatches(version, requirement string) bool {
	requirement = strings.TrimSpace(requirement)
	if requirement == "" || requirement == "*" {
		return false
	}
	for _, operator := range []string{">=", "<=", "==", ">", "<", "="} {
		if strings.HasPrefix(requirement, operator) {
			return false
		}
	}
	actual := normalizeModSemver(version)
	required := normalizeModSemver(requirement)
	if actual != "" && required != "" {
		return semver.Compare(actual, required) == 0
	}
	return strings.EqualFold(strings.TrimSpace(version), requirement)
}

func modVersionSatisfies(version, requirement string) bool {
	requirement = strings.TrimSpace(requirement)
	if requirement == "" || requirement == "*" {
		return true
	}

	operator := ">="
	for _, candidate := range []string{">=", "<=", "==", ">", "<", "="} {
		if strings.HasPrefix(requirement, candidate) {
			operator = candidate
			requirement = strings.TrimSpace(strings.TrimPrefix(requirement, candidate))
			break
		}
	}

	actual := normalizeModSemver(version)
	required := normalizeModSemver(requirement)
	if actual == "" || required == "" {
		if operator == "=" || operator == "==" {
			return strings.EqualFold(strings.TrimSpace(version), requirement)
		}
		return false
	}

	comparison := semver.Compare(actual, required)
	switch operator {
	case ">":
		return comparison > 0
	case "<":
		return comparison < 0
	case "<=":
		return comparison <= 0
	case "=", "==":
		return comparison == 0
	default:
		// Vintage Story treats a plain dependency version as the minimum
		// acceptable version.
		return comparison >= 0
	}
}

func normalizeModSemver(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(version), "v") {
		version = "v" + version
	}
	if !semver.IsValid(version) {
		return ""
	}
	return version
}

func canonicalCatalogModID(details domain.ModDetails) string {
	id := strings.TrimSpace(details.ID)
	if id == "" {
		id = strings.TrimSpace(details.Slug)
	}
	return strings.ToLower(id)
}

func isBuiltInModDependency(modID string) bool {
	switch strings.ToLower(strings.TrimSpace(modID)) {
	case "game", "survival", "creative":
		return true
	default:
		return false
	}
}

func dependencyFailureMessage(err error) string {
	if errors.Is(err, context.Canceled) {
		return "Download cancelled"
	}
	var appErr *domain.AppError
	if errors.As(err, &appErr) && appErr.Message != "" {
		return appErr.Message
	}
	return "Could not resolve or install mod dependencies"
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

func newlyDownloadedDependencies(plan []modInstallPlanItem) []modDownloadedDependencyEvent {
	dependencies := make([]modDownloadedDependencyEvent, 0)
	for _, item := range plan {
		if item.Root || !item.DownloadedNow {
			continue
		}
		dependencies = append(dependencies, modDownloadedDependencyEvent{
			ModID:   item.Downloaded.ModID,
			Name:    item.Downloaded.Name,
			Version: item.Downloaded.DownloadedVersion,
		})
	}
	return dependencies
}

func (s *Service) emitModDownloadsChanged(
	taskID string,
	modID string,
	dependencies []modDownloadedDependencyEvent,
) {
	s.emit("mods:downloads-changed", map[string]any{
		"taskId":                 taskID,
		"modId":                  modID,
		"downloadedDependencies": dependencies,
	})
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
