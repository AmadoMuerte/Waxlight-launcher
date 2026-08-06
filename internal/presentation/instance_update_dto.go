package presentation

import "github.com/waxlight/waxlight-launcher/internal/modpack"

type ModDependencyDTO struct {
	ModID       string `json:"modId"`
	Name        string `json:"name"`
	Requirement string `json:"requirement"`
}

type ModUpdateDTO struct {
	ModID            string             `json:"modId"`
	Name             string             `json:"name"`
	InstalledVersion string             `json:"installedVersion"`
	TargetVersionID  string             `json:"targetVersionId"`
	TargetVersion    string             `json:"targetVersion"`
	Status           string             `json:"status"`
	Reason           string             `json:"reason"`
	Changelog        string             `json:"changelog"`
	Compatible       bool               `json:"compatible"`
	Prerelease       bool               `json:"prerelease"`
	AddedDeps        []ModDependencyDTO `json:"addedDeps"`
	RemovedDeps      []ModDependencyDTO `json:"removedDeps"`
}

type ModUpdateSummaryDTO struct {
	TotalMods                int `json:"totalMods"`
	UpToDate                 int `json:"upToDate"`
	UpdatesAvailable         int `json:"updatesAvailable"`
	NotUpdatableLocal        int `json:"notUpdatableLocal"`
	NotUpdatableAbsent       int `json:"notUpdatableAbsent"`
	NotUpdatableCatalogError int `json:"notUpdatableCatalogError"`
	Incompatible             int `json:"incompatible"`
}

type InstanceModUpdateReportDTO struct {
	GameVersion string              `json:"gameVersion"`
	Mods        []ModUpdateDTO      `json:"mods"`
	Summary     ModUpdateSummaryDTO `json:"summary"`
}

func modDependencyDTO(dependency modpack.Dependency) ModDependencyDTO {
	return ModDependencyDTO{
		ModID:       dependency.ModID,
		Name:        dependency.Name,
		Requirement: dependency.Requirement,
	}
}

func modUpdateDTO(update modpack.ModUpdate) ModUpdateDTO {
	return ModUpdateDTO{
		ModID:            update.ModID,
		Name:             update.Name,
		InstalledVersion: update.InstalledVersion,
		TargetVersionID:  update.TargetVersionID,
		TargetVersion:    update.TargetVersion,
		Status:           string(update.Status),
		Reason:           string(update.Reason),
		Changelog:        update.Changelog,
		Compatible:       update.Compatible,
		Prerelease:       update.Prerelease,
		AddedDeps:        dependenciesDTO(update.AddedDeps),
		RemovedDeps:      dependenciesDTO(update.RemovedDeps),
	}
}

func dependenciesDTO(dependencies []modpack.Dependency) []ModDependencyDTO {
	if dependencies == nil {
		return []ModDependencyDTO{}
	}
	result := make([]ModDependencyDTO, 0, len(dependencies))
	for _, dependency := range dependencies {
		result = append(result, modDependencyDTO(dependency))
	}
	return result
}

func modUpdateSummaryDTO(summary modpack.Summary) ModUpdateSummaryDTO {
	return ModUpdateSummaryDTO{
		TotalMods:                summary.TotalMods,
		UpToDate:                 summary.UpToDate,
		UpdatesAvailable:         summary.UpdatesAvailable,
		NotUpdatableLocal:        summary.NotUpdatableLocal,
		NotUpdatableAbsent:       summary.NotUpdatableAbsent,
		NotUpdatableCatalogError: summary.NotUpdatableCatalogError,
		Incompatible:             summary.Incompatible,
	}
}

func instanceModUpdateReportDTO(report modpack.Report) InstanceModUpdateReportDTO {
	dto := InstanceModUpdateReportDTO{
		GameVersion: report.Build.GameVersion,
		Summary:     modUpdateSummaryDTO(report.Summary),
		Mods:        make([]ModUpdateDTO, 0, len(report.Mods)),
	}
	for _, update := range report.Mods {
		dto.Mods = append(dto.Mods, modUpdateDTO(update))
	}
	return dto
}
