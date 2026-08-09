package presentation

import "github.com/waxlight/waxlight-launcher/internal/domain"

type ModSummaryDTO struct {
	ID              string   `json:"id"`
	Slug            string   `json:"slug,omitempty"`
	Name            string   `json:"name"`
	AuthorName      string   `json:"authorName"`
	Summary         string   `json:"summary"`
	ImageURL        string   `json:"imageUrl,omitempty"`
	Side            string   `json:"side"`
	LatestVersion   string   `json:"latestVersion,omitempty"`
	GameVersions    []string `json:"gameVersions"`
	Downloads       int64    `json:"downloads"`
	CreatedAt       *string  `json:"createdAt,omitempty"`
	UpdatedAt       *string  `json:"updatedAt,omitempty"`
	Tags            []string `json:"tags"`
	IsDownloaded    bool     `json:"isDownloaded"`
	IsInstalled     bool     `json:"isInstalled"`
	UpdateAvailable bool     `json:"updateAvailable"`
}

func modSummaryDTO(mod domain.ModSummary) ModSummaryDTO {
	dto := ModSummaryDTO{
		ID: mod.ID, Slug: mod.Slug, Name: mod.Name,
		AuthorName: mod.AuthorName, Summary: mod.Summary,
		ImageURL: mod.ImageURL, Side: string(mod.Side),
		LatestVersion: mod.LatestVersion,
		GameVersions:  nonNilStrings(mod.GameVersions), Downloads: mod.Downloads,
		Tags: nonNilStrings(mod.Tags), IsDownloaded: mod.IsDownloaded,
		IsInstalled: mod.IsInstalled, UpdateAvailable: mod.UpdateAvailable,
	}
	if mod.CreatedAt != nil {
		value := iso(*mod.CreatedAt)
		dto.CreatedAt = &value
	}
	if mod.UpdatedAt != nil {
		value := iso(*mod.UpdatedAt)
		dto.UpdatedAt = &value
	}
	return dto
}

type ModScreenshotDTO struct {
	URL     string `json:"url"`
	Caption string `json:"caption,omitempty"`
}

type ModVersionDTO struct {
	ID           string   `json:"id"`
	Version      string   `json:"version"`
	GameVersions []string `json:"gameVersions"`
	ReleaseType  string   `json:"releaseType"`
	FileName     string   `json:"fileName"`
	FileSize     int64    `json:"fileSize"`
	PublishedAt  *string  `json:"publishedAt,omitempty"`
	Changelog    string   `json:"changelog,omitempty"`
}

type ModDetailsDTO struct {
	ModSummaryDTO
	Description string             `json:"description"`
	Screenshots []ModScreenshotDTO `json:"screenshots"`
	Versions    []ModVersionDTO    `json:"versions"`
	WebsiteURL  string             `json:"websiteUrl,omitempty"`
	SourceURL   string             `json:"sourceUrl,omitempty"`
	License     string             `json:"license,omitempty"`
}

func modDetailsDTO(mod domain.ModDetails) ModDetailsDTO {
	dto := ModDetailsDTO{
		ModSummaryDTO: modSummaryDTO(mod.ModSummary),
		Description:   mod.Description,
		Screenshots:   []ModScreenshotDTO{},
		Versions:      []ModVersionDTO{},
		WebsiteURL:    mod.WebsiteURL,
		SourceURL:     mod.SourceURL,
		License:       mod.License,
	}
	for _, screenshot := range mod.Screenshots {
		dto.Screenshots = append(dto.Screenshots, ModScreenshotDTO{
			URL: screenshot.URL, Caption: screenshot.Caption,
		})
	}
	for _, version := range mod.Versions {
		item := ModVersionDTO{
			ID: version.ID, Version: version.Version,
			GameVersions: nonNilStrings(version.GameVersions),
			ReleaseType:  version.ReleaseType, FileName: version.FileName,
			FileSize: version.FileSize, Changelog: version.Changelog,
		}
		if version.PublishedAt != nil {
			value := iso(*version.PublishedAt)
			item.PublishedAt = &value
		}
		dto.Versions = append(dto.Versions, item)
	}
	return dto
}

type ModTagDTO struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type ModSearchResultDTO struct {
	Items      []ModSummaryDTO `json:"items"`
	Page       int             `json:"page"`
	PageSize   int             `json:"pageSize"`
	TotalItems int             `json:"totalItems"`
	TotalPages int             `json:"totalPages"`
	HasNext    bool            `json:"hasNext"`
}

type LocalModLinkDTO struct {
	Path            string `json:"path,omitempty"`
	Name            string `json:"name"`
	Version         string `json:"version"`
	FileName        string `json:"fileName"`
	ModID           string `json:"modId,omitempty"`
	VersionID       string `json:"versionId,omitempty"`
	Slug            string `json:"slug,omitempty"`
	LatestVersion   string `json:"latestVersion,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable"`
	Reason          string `json:"reason,omitempty"`
}

func localModLinkDTO(link domain.LocalModLink) LocalModLinkDTO {
	return LocalModLinkDTO{
		Path:            link.Path,
		Name:            link.Name,
		Version:         link.Version,
		FileName:        link.FileName,
		ModID:           link.ModID,
		VersionID:       link.VersionID,
		Slug:            link.Slug,
		LatestVersion:   link.LatestVersion,
		UpdateAvailable: link.UpdateAvailable,
		Reason:          link.Reason,
	}
}

func localModLinksDTO(links []domain.LocalModLink) []LocalModLinkDTO {
	dtos := make([]LocalModLinkDTO, 0, len(links))
	for _, link := range links {
		dtos = append(dtos, localModLinkDTO(link))
	}
	return dtos
}

type LinkLocalModsResultDTO struct {
	Linked     []LocalModLinkDTO `json:"linked"`
	NotMatched []LocalModLinkDTO `json:"notMatched"`
	Failed     []LocalModLinkDTO `json:"failed"`
}

func linkLocalModsResultDTO(result domain.LinkLocalModsResult) LinkLocalModsResultDTO {
	return LinkLocalModsResultDTO{
		Linked:     localModLinksDTO(result.Linked),
		NotMatched: localModLinksDTO(result.NotMatched),
		Failed:     localModLinksDTO(result.Failed),
	}
}

type UploadModsResultDTO struct {
	Linked     []LocalModLinkDTO `json:"linked"`
	NotMatched []LocalModLinkDTO `json:"notMatched"`
	Skipped    []string          `json:"skipped"`
	Failed     []LocalModLinkDTO `json:"failed"`
}

func uploadModsResultDTO(result domain.UploadModsResult) UploadModsResultDTO {
	return UploadModsResultDTO{
		Linked:     localModLinksDTO(result.Linked),
		NotMatched: localModLinksDTO(result.NotMatched),
		Skipped:    nonNilStrings(result.Skipped),
		Failed:     localModLinksDTO(result.Failed),
	}
}

type InstalledModInstanceDTO struct {
	InstanceID   string `json:"instanceId"`
	InstanceName string `json:"instanceName"`
	Version      string `json:"version"`
	Enabled      bool   `json:"enabled"`
}

type DownloadedModDTO struct {
	ModID              string                    `json:"modId"`
	Slug               string                    `json:"slug,omitempty"`
	Name               string                    `json:"name"`
	AuthorName         string                    `json:"authorName"`
	ImageURL           string                    `json:"imageUrl,omitempty"`
	Side               string                    `json:"side"`
	VersionID          string                    `json:"versionId"`
	DownloadedVersion  string                    `json:"downloadedVersion"`
	GameVersions       []string                  `json:"gameVersions"`
	FileName           string                    `json:"fileName"`
	FileSize           int64                     `json:"fileSize"`
	DownloadedAt       string                    `json:"downloadedAt"`
	InstalledInstances []InstalledModInstanceDTO `json:"installedInstances"`
	LatestVersion      string                    `json:"latestVersion,omitempty"`
	UpdateAvailable    bool                      `json:"updateAvailable"`
}

func downloadedModDTO(mod domain.DownloadedMod) DownloadedModDTO {
	dto := DownloadedModDTO{
		ModID: mod.ModID, Slug: mod.Slug, Name: mod.Name,
		AuthorName: mod.AuthorName, ImageURL: mod.ImageURL,
		Side: string(mod.Side), VersionID: mod.VersionID,
		DownloadedVersion: mod.DownloadedVersion,
		GameVersions:      nonNilStrings(mod.GameVersions), FileName: mod.FileName,
		FileSize: mod.FileSize, DownloadedAt: iso(mod.DownloadedAt),
		InstalledInstances: []InstalledModInstanceDTO{},
		LatestVersion:      mod.LatestVersion, UpdateAvailable: mod.UpdateAvailable,
	}
	for _, installed := range mod.InstalledInstances {
		dto.InstalledInstances = append(dto.InstalledInstances, InstalledModInstanceDTO{
			InstanceID: installed.InstanceID, InstanceName: installed.InstanceName,
			Version: installed.Version, Enabled: installed.Enabled,
		})
	}
	return dto
}

type ModInstallationResultDTO struct {
	InstanceID   string `json:"instanceId"`
	InstanceName string `json:"instanceName"`
	Installed    bool   `json:"installed"`
	Message      string `json:"message"`
}

type ModInstallResultDTO struct {
	TaskID        string                     `json:"taskId"`
	Downloaded    DownloadedModDTO           `json:"downloaded"`
	Installations []ModInstallationResultDTO `json:"installations"`
}

type BatchModInstallResultDTO struct {
	ModID     string              `json:"modId"`
	VersionID string              `json:"versionId"`
	Result    ModInstallResultDTO `json:"result"`
	Error     string              `json:"error,omitempty"`
}

type DownloadedModCleanupResultDTO struct {
	RemovedCount int   `json:"removedCount"`
	FreedBytes   int64 `json:"freedBytes"`
}

func downloadedModCleanupResultDTO(result domain.DownloadedModCleanupResult) DownloadedModCleanupResultDTO {
	return DownloadedModCleanupResultDTO{RemovedCount: result.RemovedCount, FreedBytes: result.FreedBytes}
}

func batchModInstallResultsDTO(results []domain.BatchModInstallResult) []BatchModInstallResultDTO {
	dtos := make([]BatchModInstallResultDTO, 0, len(results))
	for _, result := range results {
		dtos = append(dtos, BatchModInstallResultDTO{
			ModID:     result.ModID,
			VersionID: result.VersionID,
			Result:    modInstallResultDTO(result.Result),
			Error:     result.Error,
		})
	}
	return dtos
}

func modInstallResultDTO(result domain.ModInstallResult) ModInstallResultDTO {
	dto := ModInstallResultDTO{
		TaskID: result.TaskID, Downloaded: downloadedModDTO(result.Downloaded),
		Installations: []ModInstallationResultDTO{},
	}
	for _, item := range result.Installations {
		dto.Installations = append(dto.Installations, ModInstallationResultDTO{
			InstanceID: item.InstanceID, InstanceName: item.InstanceName,
			Installed: item.Installed, Message: item.Message,
		})
	}
	return dto
}
