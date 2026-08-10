package presentation

import (
	"time"

	"github.com/waxlight/waxlight-launcher/internal/application"
	"github.com/waxlight/waxlight-launcher/internal/domain"
)

func iso(value time.Time) string {
	return value.Format(time.RFC3339Nano)
}

type AccountDTO struct {
	ID              string  `json:"id"`
	Username        string  `json:"username"`
	DisplayName     string  `json:"displayName"`
	Email           string  `json:"email"`
	Status          string  `json:"status"`
	IsDefault       bool    `json:"isDefault"`
	LastValidatedAt *string `json:"lastValidatedAt,omitempty"`
}

func accountDTO(account domain.Account) AccountDTO {
	result := AccountDTO{
		ID:          account.ID,
		Username:    account.Username,
		DisplayName: account.DisplayName,
		Email:       account.Email,
		Status:      string(account.Status),
		IsDefault:   account.IsDefault,
	}
	if account.LastValidatedAt != nil {
		value := iso(*account.LastValidatedAt)
		result.LastValidatedAt = &value
	}
	return result
}

type LoginResultDTO struct {
	Status  string      `json:"status"`
	Account *AccountDTO `json:"account,omitempty"`
	FlowID  string      `json:"flowId,omitempty"`
	Message string      `json:"message,omitempty"`
}

func loginResultDTO(result application.LoginResult) LoginResultDTO {
	dto := LoginResultDTO{
		Status:  result.Status,
		FlowID:  result.FlowID,
		Message: result.Message,
	}
	if result.Account != nil {
		account := accountDTO(*result.Account)
		dto.Account = &account
	}
	return dto
}

type GameVersionDTO struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Channel         string `json:"channel"`
	Platform        string `json:"platform"`
	Architecture    string `json:"architecture"`
	InstallationDir string `json:"installationDir"`
	ExecutablePath  string `json:"executablePath"`
	Status          string `json:"status"`
	SizeBytes       int64  `json:"sizeBytes"`
	InstalledAt     string `json:"installedAt"`
}

func versionDTO(version domain.GameVersion) GameVersionDTO {
	return GameVersionDTO{
		ID:              version.ID,
		Name:            version.Name,
		Channel:         version.Channel,
		Platform:        version.Platform,
		Architecture:    version.Architecture,
		InstallationDir: version.InstallationDir,
		ExecutablePath:  version.ExecutablePath,
		Status:          version.Status,
		SizeBytes:       version.SizeBytes,
		InstalledAt:     iso(version.InstalledAt),
	}
}

type AvailableGameVersionDTO struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Channel       string  `json:"channel"`
	Platform      string  `json:"platform"`
	Architecture  string  `json:"architecture"`
	DownloadSize  int64   `json:"downloadSize"`
	Latest        bool    `json:"latest"`
	Installed     bool    `json:"installed"`
	InstallStatus *string `json:"installStatus,omitempty"`
}

func availableVersionDTO(
	version domain.AvailableGameVersion,
) AvailableGameVersionDTO {
	return AvailableGameVersionDTO{
		ID:            version.ID,
		Name:          version.Name,
		Channel:       version.Channel,
		Platform:      version.Platform,
		Architecture:  version.Architecture,
		DownloadSize:  version.DownloadSize,
		Latest:        version.Latest,
		Installed:     version.Installed,
		InstallStatus: version.InstallStatus,
	}
}

type InstanceDTO struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	GameVersionID    string   `json:"gameVersionId"`
	DefaultAccountID *string  `json:"defaultAccountId,omitempty"`
	Directory        string   `json:"directory"`
	Status           string   `json:"status"`
	LaunchArguments  []string `json:"launchArguments"`
	LastPlayedAt     *string  `json:"lastPlayedAt,omitempty"`
	CreatedAt        string   `json:"createdAt"`
	EnabledModCount  int      `json:"enabledModCount"`
	TotalModCount    int      `json:"totalModCount"`
	PlaytimeSeconds  int64    `json:"playtimeSeconds"`
}

func instanceDTO(instance domain.Instance) InstanceDTO {
	launchArguments := instance.LaunchArguments
	if launchArguments == nil {
		launchArguments = []string{}
	}

	result := InstanceDTO{
		ID:               instance.ID,
		Name:             instance.Name,
		Description:      instance.Description,
		GameVersionID:    instance.GameVersionID,
		DefaultAccountID: instance.DefaultAccountID,
		Directory:        instance.Directory,
		Status:           instance.Status,
		LaunchArguments:  launchArguments,
		CreatedAt:        iso(instance.CreatedAt),
	}
	if instance.LastPlayedAt != nil {
		value := iso(*instance.LastPlayedAt)
		result.LastPlayedAt = &value
	}
	return result
}

type FavoriteServerDTO struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Address    string  `json:"address"`
	InstanceID *string `json:"instanceId,omitempty"`
}

func favoriteServerDTO(server domain.FavoriteServer) FavoriteServerDTO {
	return FavoriteServerDTO{ID: server.ID, Name: server.Name, Address: server.Address, InstanceID: server.InstanceID}
}

type PublicServerDTO struct {
	Name              string `json:"name"`
	Address           string `json:"address"`
	Description       string `json:"description"`
	Players           int    `json:"players"`
	ModCount          int    `json:"modCount"`
	RequiresWhitelist bool   `json:"requiresWhitelist"`
	AccessRestricted  bool   `json:"accessRestricted"`
	Joinable          bool   `json:"joinable"`
}

func publicServerDTO(server domain.PublicServer) PublicServerDTO {
	return PublicServerDTO{
		Name: server.Name, Address: server.Address, Description: server.Description,
		Players: server.Players, ModCount: server.ModCount,
		RequiresWhitelist: server.RequiresWhitelist, AccessRestricted: server.PasswordProtected,
		Joinable: server.Joinable,
	}
}

type InstalledModDTO struct {
	ID          string `json:"id"`
	InstanceID  string `json:"instanceId"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	FileName    string `json:"fileName"`
	FilePath    string `json:"filePath"`
	Enabled     bool   `json:"enabled"`
	Managed     bool   `json:"managed"`
	Source      string `json:"source"`
	SizeBytes   int64  `json:"sizeBytes"`
	InstalledAt string `json:"installedAt"`
}

type ModDeletePreviewDTO struct {
	ModID        string            `json:"modId"`
	ModName      string            `json:"modName"`
	Dependencies []InstalledModDTO `json:"dependencies"`
}

func modDTO(mod domain.InstalledMod) InstalledModDTO {
	return InstalledModDTO{
		ID:          mod.ID,
		InstanceID:  mod.InstanceID,
		Name:        mod.Name,
		Version:     mod.Version,
		FileName:    mod.FileName,
		FilePath:    mod.FilePath,
		Enabled:     mod.Enabled,
		Managed:     mod.Managed,
		Source:      mod.Source,
		SizeBytes:   mod.SizeBytes,
		InstalledAt: iso(mod.InstalledAt),
	}
}

type OperationDTO struct {
	ID             string            `json:"id"`
	Type           string            `json:"type"`
	ResourceID     *string           `json:"resourceId,omitempty"`
	Title          string            `json:"title"`
	TitleKey       string            `json:"titleKey,omitempty"`
	TitleParams    map[string]string `json:"titleParams,omitempty"`
	Status         string            `json:"status"`
	Progress       float64           `json:"progress"`
	CurrentBytes   int64             `json:"currentBytes"`
	TotalBytes     int64             `json:"totalBytes"`
	BytesPerSecond int64             `json:"bytesPerSecond"`
	ErrorCode      *string           `json:"errorCode,omitempty"`
	ErrorMessage   *string           `json:"errorMessage,omitempty"`
	CreatedAt      string            `json:"createdAt"`
	StartedAt      *string           `json:"startedAt,omitempty"`
	FinishedAt     *string           `json:"finishedAt,omitempty"`
}

func operationDTO(operation domain.Operation) OperationDTO {
	result := OperationDTO{
		ID:             operation.ID,
		Type:           operation.Type,
		ResourceID:     operation.ResourceID,
		Title:          operation.Title,
		TitleKey:       operation.TitleKey,
		TitleParams:    operation.TitleParams,
		Status:         operation.Status,
		Progress:       operation.Progress,
		CurrentBytes:   operation.CurrentBytes,
		TotalBytes:     operation.TotalBytes,
		BytesPerSecond: operation.BytesPerSecond,
		ErrorCode:      operation.ErrorCode,
		ErrorMessage:   operation.ErrorMessage,
		CreatedAt:      iso(operation.CreatedAt),
	}
	if operation.StartedAt != nil {
		value := iso(*operation.StartedAt)
		result.StartedAt = &value
	}
	if operation.FinishedAt != nil {
		value := iso(*operation.FinishedAt)
		result.FinishedAt = &value
	}
	return result
}

type PlaySessionDTO struct {
	ID              string  `json:"id"`
	InstanceID      string  `json:"instanceId"`
	AccountID       *string `json:"accountId,omitempty"`
	VersionID       string  `json:"versionId"`
	StartedAt       string  `json:"startedAt"`
	EndedAt         *string `json:"endedAt,omitempty"`
	DurationSeconds int64   `json:"durationSeconds"`
	ExitCode        *int    `json:"exitCode,omitempty"`
	Crashed         bool    `json:"crashed"`
	Recovered       bool    `json:"recovered"`
}

func sessionDTO(session domain.PlaySession) PlaySessionDTO {
	result := PlaySessionDTO{
		ID:              session.ID,
		InstanceID:      session.InstanceID,
		AccountID:       session.AccountID,
		VersionID:       session.VersionID,
		StartedAt:       iso(session.StartedAt),
		DurationSeconds: session.DurationSec,
		ExitCode:        session.ExitCode,
		Crashed:         session.Crashed,
		Recovered:       session.Recovered,
	}
	if session.EndedAt != nil {
		value := iso(*session.EndedAt)
		result.EndedAt = &value
	}
	return result
}

type StatisticsDTO struct {
	TotalPlaytimeSeconds  int64            `json:"totalPlaytimeSeconds"`
	LaunchCount           int              `json:"launchCount"`
	AverageSessionSeconds int64            `json:"averageSessionSeconds"`
	MostPlayedInstanceID  *string          `json:"mostPlayedInstanceId,omitempty"`
	RecentSessions        []PlaySessionDTO `json:"recentSessions"`
}

func statisticsDTO(statistics application.Statistics) StatisticsDTO {
	result := StatisticsDTO{
		TotalPlaytimeSeconds:  statistics.TotalPlaytimeSeconds,
		LaunchCount:           statistics.LaunchCount,
		AverageSessionSeconds: statistics.AverageSessionSeconds,
		MostPlayedInstanceID:  statistics.MostPlayedInstanceID,
		RecentSessions:        []PlaySessionDTO{},
	}
	for _, session := range statistics.RecentSessions {
		result.RecentSessions = append(
			result.RecentSessions,
			sessionDTO(session),
		)
	}
	return result
}

type SettingsDTO struct {
	Language                 string   `json:"language"`
	DownloadsParallel        int      `json:"downloadsParallel"`
	ConfirmDeletion          bool     `json:"confirmDeletion"`
	GlobalLaunchArguments    []string `json:"globalLaunchArguments"`
	CheckForUpdates          bool     `json:"checkForUpdates"`
	UpdateChannel            string   `json:"updateChannel"`
	SkippedUpdateVersion     string   `json:"skippedUpdateVersion"`
	TelemetryEnabled         bool     `json:"telemetryEnabled"`
	AutomaticSafetySnapshots bool     `json:"automaticSafetySnapshots"`
}

func settingsDTO(settings domain.Settings) SettingsDTO {
	launchArguments := settings.GlobalLaunchArguments
	if launchArguments == nil {
		launchArguments = []string{}
	}

	return SettingsDTO{
		Language:                 settings.Language,
		DownloadsParallel:        settings.DownloadsParallel,
		ConfirmDeletion:          settings.ConfirmDeletion,
		GlobalLaunchArguments:    launchArguments,
		CheckForUpdates:          settings.CheckForUpdates,
		UpdateChannel:            settings.UpdateChannel,
		SkippedUpdateVersion:     settings.SkippedUpdateVersion,
		TelemetryEnabled:         settings.TelemetryEnabled,
		AutomaticSafetySnapshots: settings.AutomaticSafetySnapshots,
	}
}

type DataFolderDTO struct {
	CurrentPath string `json:"currentPath"`
	DefaultPath string `json:"defaultPath"`
	LastError   string `json:"lastError"`
}

type DataFolderProgressDTO struct {
	CopiedBytes int64   `json:"copiedBytes"`
	TotalBytes  int64   `json:"totalBytes"`
	Progress    float64 `json:"progress"`
	Phase       string  `json:"phase"`
}
