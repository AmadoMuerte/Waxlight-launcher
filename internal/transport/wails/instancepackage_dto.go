package wails

import "github.com/AmadoMuerte/Waxlight-launcher/internal/instances"

// PackageAuthorDTO is the frontend-safe representation of package author.
type PackageAuthorDTO struct {
	Name     string `json:"name,omitempty"`
	Homepage string `json:"homepage,omitempty"`
	Source   string `json:"source,omitempty"`
}

func packageAuthorDTO(author *instances.PackageAuthor) *PackageAuthorDTO {
	if author == nil {
		return nil
	}
	return &PackageAuthorDTO{
		Name:     author.Name,
		Homepage: author.Homepage,
		Source:   author.Source,
	}
}

// PackageGameVersionDTO is the frontend-safe representation of package game version.
type PackageGameVersionDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PackageModDTO is the frontend-safe representation of package mod.
type PackageModDTO struct {
	ModID       string `json:"modId,omitempty"`
	VersionID   string `json:"versionId,omitempty"`
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	FileName    string `json:"fileName"`
	Source      string `json:"source"`
	Checksum    string `json:"checksum,omitempty"`
	DownloadURL string `json:"downloadUrl,omitempty"`
	Enabled     bool   `json:"enabled"`
}

func packageModDTO(mod instances.PackageMod) PackageModDTO {
	return PackageModDTO{
		ModID:       mod.ModID,
		VersionID:   mod.VersionID,
		Name:        mod.Name,
		Version:     mod.Version,
		FileName:    mod.FileName,
		Source:      string(mod.Source),
		Checksum:    mod.Checksum,
		DownloadURL: mod.DownloadURL,
		Enabled:     mod.Enabled,
	}
}

// PackageManifestDTO describes the portable instance metadata included in an export.
type PackageManifestDTO struct {
	SchemaVersion   int                   `json:"schemaVersion"`
	Name            string                `json:"name"`
	Description     string                `json:"description,omitempty"`
	Author          *PackageAuthorDTO     `json:"author,omitempty"`
	GameVersion     PackageGameVersionDTO `json:"gameVersion"`
	LaunchArguments []string              `json:"launchArguments"`
	Mods            []PackageModDTO       `json:"mods"`
	ConfigFiles     []string              `json:"configFiles"`
	HasIcon         bool                  `json:"hasIcon"`
}

func packageManifestDTO(manifest instances.PackageManifest) PackageManifestDTO {
	launchArguments := manifest.LaunchArguments
	if launchArguments == nil {
		launchArguments = []string{}
	}
	mods := make([]PackageModDTO, 0, len(manifest.Mods))
	for _, mod := range manifest.Mods {
		mods = append(mods, packageModDTO(mod))
	}
	configFiles := manifest.ConfigFiles
	if configFiles == nil {
		configFiles = []string{}
	}
	return PackageManifestDTO{
		SchemaVersion:   manifest.SchemaVersion,
		Name:            manifest.Name,
		Description:     manifest.Description,
		Author:          packageAuthorDTO(manifest.Author),
		GameVersion:     PackageGameVersionDTO{ID: manifest.GameVersion.ID, Name: manifest.GameVersion.Name},
		LaunchArguments: launchArguments,
		Mods:            mods,
		ConfigFiles:     configFiles,
		HasIcon:         manifest.HasIcon,
	}
}

// PackageModCheckDTO is the frontend-safe representation of package mod check.
type PackageModCheckDTO struct {
	ModID       string `json:"modId,omitempty"`
	VersionID   string `json:"versionId,omitempty"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Source      string `json:"source"`
	Enabled     bool   `json:"enabled"`
	Status      string `json:"status"`
	Message     string `json:"message,omitempty"`
	HasEmbedded bool   `json:"hasEmbedded,omitempty"`
}

func packageModCheckDTO(check instances.PackageModCheck) PackageModCheckDTO {
	return PackageModCheckDTO{
		ModID:       check.ModID,
		VersionID:   check.VersionID,
		Name:        check.Name,
		Version:     check.Version,
		Source:      string(check.Source),
		Enabled:     check.Enabled,
		Status:      string(check.Status),
		Message:     check.Message,
		HasEmbedded: check.HasEmbedded,
	}
}

// PackageInspectionDTO reports package contents and compatibility before import.
type PackageInspectionDTO struct {
	Path            string                `json:"path"`
	SchemaVersion   int                   `json:"schemaVersion"`
	Name            string                `json:"name"`
	Description     string                `json:"description,omitempty"`
	Author          *PackageAuthorDTO     `json:"author,omitempty"`
	GameVersion     PackageGameVersionDTO `json:"gameVersion"`
	VersionStatus   string                `json:"versionStatus"`
	LaunchArguments []string              `json:"launchArguments"`
	Mods            []PackageModCheckDTO  `json:"mods"`
	ConfigFiles     []string              `json:"configFiles"`
	HasIcon         bool                  `json:"hasIcon"`
	TotalSize       int64                 `json:"totalSize"`
	UnverifiedFiles int                   `json:"unverifiedFiles"`
	Warnings        []string              `json:"warnings"`
}

func packageInspectionDTO(inspection instances.PackageInspection) PackageInspectionDTO {
	mods := make([]PackageModCheckDTO, 0, len(inspection.Mods))
	for _, mod := range inspection.Mods {
		mods = append(mods, packageModCheckDTO(mod))
	}
	configFiles := inspection.ConfigFiles
	if configFiles == nil {
		configFiles = []string{}
	}
	warnings := inspection.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	launchArguments := inspection.LaunchArguments
	if launchArguments == nil {
		launchArguments = []string{}
	}
	return PackageInspectionDTO{
		Path:            inspection.Path,
		SchemaVersion:   inspection.SchemaVersion,
		Name:            inspection.Name,
		Description:     inspection.Description,
		Author:          packageAuthorDTO(inspection.Author),
		GameVersion:     PackageGameVersionDTO{ID: inspection.GameVersion.ID, Name: inspection.GameVersion.Name},
		VersionStatus:   string(inspection.VersionStatus),
		LaunchArguments: launchArguments,
		Mods:            mods,
		ConfigFiles:     configFiles,
		HasIcon:         inspection.HasIcon,
		TotalSize:       inspection.TotalSize,
		UnverifiedFiles: inspection.UnverifiedFiles,
		Warnings:        warnings,
	}
}

// ImportedModResultDTO is the frontend-safe representation of imported mod result.
type ImportedModResultDTO struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// ImportReportDTO is the frontend-safe representation of import report.
type ImportReportDTO struct {
	InstanceID    string                 `json:"instanceId"`
	InstanceName  string                 `json:"instanceName"`
	GameVersionID string                 `json:"gameVersionId"`
	Mods          []ImportedModResultDTO `json:"mods"`
	Warnings      []string               `json:"warnings"`
}

func importReportDTO(report instances.ImportReport) ImportReportDTO {
	mods := make([]ImportedModResultDTO, 0, len(report.Mods))
	for _, mod := range report.Mods {
		mods = append(mods, ImportedModResultDTO{
			Name:    mod.Name,
			Version: mod.Version,
			Status:  mod.Status,
			Message: mod.Message,
		})
	}
	warnings := report.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return ImportReportDTO{
		InstanceID:    report.InstanceID,
		InstanceName:  report.InstanceName,
		GameVersionID: report.GameVersionID,
		Mods:          mods,
		Warnings:      warnings,
	}
}
