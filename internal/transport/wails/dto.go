package wails

import (
	"time"

	"github.com/AmadoMuerte/Waxlight-launcher/internal/accounts"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/instances"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/mods"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/operations"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/optimum"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/sessions"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/settings"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/statistics"
	"github.com/AmadoMuerte/Waxlight-launcher/internal/versions"
)

func iso(value time.Time) string {
	return value.Format(time.RFC3339Nano)
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// AccountDTO summarizes a saved account without exposing credentials.
type AccountDTO struct {
	ID              string  `json:"id"`
	Username        string  `json:"username"`
	DisplayName     string  `json:"displayName"`
	Email           string  `json:"email"`
	Status          string  `json:"status"`
	IsDefault       bool    `json:"isDefault"`
	LastValidatedAt *string `json:"lastValidatedAt,omitempty"`
}

func accountDTO(account accounts.Account) AccountDTO {
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

// LoginResultDTO reports sign-in completion or the next verification step without exposing credentials.
type LoginResultDTO struct {
	Status  string      `json:"status"`
	Account *AccountDTO `json:"account,omitempty"`
	FlowID  string      `json:"flowId,omitempty"`
	Message string      `json:"message,omitempty"`
}

func loginResultDTO(result accounts.LoginResult) LoginResultDTO {
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

// GameVersionDTO describes an installed game version for library display.
type GameVersionDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Channel is the release channel, such as stable or prerelease.
	Channel string `json:"channel"`
	// Platform and Architecture identify the build the version supports.
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
	// InstallationDir is the version directory relative to the launcher data root.
	InstallationDir string `json:"installationDir"`
	// ExecutablePath is the game executable used to launch this version.
	ExecutablePath string `json:"executablePath"`
	// Status reports whether the version is installed or imported.
	Status string `json:"status"`
	// SizeBytes is the on-disk installation size.
	SizeBytes int64 `json:"sizeBytes"`
	// InstalledAt is the timestamp of the installation.
	InstalledAt string `json:"installedAt"`
}

func versionDTO(version versions.GameVersion) GameVersionDTO {
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

// AvailableGameVersionDTO describes a downloadable game release for version selection.
type AvailableGameVersionDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Channel is the release channel, such as stable or prerelease.
	Channel string `json:"channel"`
	// Platform and Architecture identify the build this release provides.
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
	// DownloadSize is the expected archive size in bytes.
	DownloadSize int64 `json:"downloadSize"`
	// Latest reports whether this is the newest release of its channel.
	Latest bool `json:"latest"`
	// Installed reports whether this exact release is already present.
	Installed bool `json:"installed"`
	// InstallStatus carries the reason when the release cannot be installed.
	InstallStatus *string `json:"installStatus,omitempty"`
}

func availableVersionDTO(
	version versions.AvailableGameVersion,
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

// InstanceDTO describes a managed game instance for library and detail views.
type InstanceDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Description is the user-visible instance description.
	Description string `json:"description"`
	// GameVersionID selects the installed game version used by the instance.
	GameVersionID string `json:"gameVersionId"`
	// GameClient selects the runtime implementation: vanilla or optimum.
	GameClient string `json:"gameClient"`
	// DefaultAccountID is the account used when the instance launches without an explicit choice.
	DefaultAccountID *string `json:"defaultAccountId,omitempty"`
	// Directory is the instance data directory relative to the launcher data root.
	Directory string `json:"directory"`
	// Status is the current instance lifecycle state.
	Status string `json:"status"`
	// LaunchArguments are extra command-line arguments appended at launch.
	LaunchArguments []string `json:"launchArguments"`
	// EnvironmentVariables are additional environment values applied at launch.
	EnvironmentVariables map[string]string `json:"environmentVariables"`
	// IsPinned reports whether the instance is pinned in the library.
	IsPinned bool `json:"isPinned"`
	// LastPlayedAt is the timestamp of the most recent launch, when known.
	LastPlayedAt *string `json:"lastPlayedAt,omitempty"`
	CreatedAt    string  `json:"createdAt"`
	// EnabledModCount and TotalModCount summarize the installed mods.
	EnabledModCount int `json:"enabledModCount"`
	TotalModCount   int `json:"totalModCount"`
	// PlaytimeSeconds is the accumulated play time across sessions.
	PlaytimeSeconds int64 `json:"playtimeSeconds"`
	// CoverURL serves the instance cover image, when one is set.
	CoverURL *string `json:"coverUrl,omitempty"`
}

func instanceDTO(instance instances.Instance) InstanceDTO {
	launchArguments := instance.LaunchArguments
	if launchArguments == nil {
		launchArguments = []string{}
	}
	environmentVariables := instance.EnvironmentVariables
	if environmentVariables == nil {
		environmentVariables = map[string]string{}
	}

	result := InstanceDTO{
		ID:                   instance.ID,
		Name:                 instance.Name,
		Description:          instance.Description,
		GameVersionID:        instance.GameVersionID,
		GameClient:           string(instance.GameClient),
		DefaultAccountID:     instance.DefaultAccountID,
		Directory:            instance.Directory,
		Status:               instance.Status,
		LaunchArguments:      launchArguments,
		EnvironmentVariables: environmentVariables,
		IsPinned:             instance.IsPinned,
		CreatedAt:            iso(instance.CreatedAt),
	}
	if instance.LastPlayedAt != nil {
		value := iso(*instance.LastPlayedAt)
		result.LastPlayedAt = &value
	}
	if instance.CoverPath != nil {
		value := "/instance-covers/" + instance.ID + "?v=" + iso(instance.UpdatedAt)
		result.CoverURL = &value
	}
	return result
}

// InstalledModDTO describes an instance mod and its enabled and update state.
type InstalledModDTO struct {
	ID           string `json:"id"`
	InstanceID   string `json:"instanceId"`
	Name         string `json:"name"`
	Version      string `json:"version"`
	FileName     string `json:"fileName"`
	FilePath     string `json:"filePath"`
	Enabled      bool   `json:"enabled"`
	Managed      bool   `json:"managed"`
	Source       string `json:"source"`
	UpdatePolicy string `json:"updatePolicy"`
	SizeBytes    int64  `json:"sizeBytes"`
	InstalledAt  string `json:"installedAt"`
}

// ModDeletePreviewDTO shows which dependent mods would be affected by removal.
type ModDeletePreviewDTO struct {
	ModID        string            `json:"modId"`
	ModName      string            `json:"modName"`
	Dependencies []InstalledModDTO `json:"dependencies"`
}

func modDTO(mod mods.InstalledMod) InstalledModDTO {
	return InstalledModDTO{
		ID:           mod.ID,
		InstanceID:   mod.InstanceID,
		Name:         mod.Name,
		Version:      mod.Version,
		FileName:     mod.FileName,
		FilePath:     mod.FilePath,
		Enabled:      mod.Enabled,
		Managed:      mod.Managed,
		Source:       mod.Source,
		UpdatePolicy: string(mods.NormalizeUpdatePolicy(mod.UpdatePolicy)),
		SizeBytes:    mod.SizeBytes,
		InstalledAt:  iso(mod.InstalledAt),
	}
}

// OperationDTO reports progress and outcome for a background launcher task.
type OperationDTO struct {
	ID string `json:"id"`
	// Type identifies the operation kind, such as a version install or snapshot.
	Type string `json:"type"`
	// ResourceID names the instance or game version the operation targets.
	ResourceID *string `json:"resourceId,omitempty"`
	// Title is the human-readable operation title, used as the i18n fallback.
	Title string `json:"title"`
	// TitleKey is the i18n key used to localize the operation title.
	TitleKey string `json:"titleKey,omitempty"`
	// TitleParams are the interpolation values for the localized title.
	TitleParams map[string]string `json:"titleParams,omitempty"`
	// Status is the operation lifecycle state: queued, running, completed, failed, or cancelled.
	Status string `json:"status"`
	// Progress is the completion ratio normalized from 0 to 1.
	Progress float64 `json:"progress"`
	// CurrentBytes is the transferred byte count, and TotalBytes the expected total.
	CurrentBytes int64 `json:"currentBytes"`
	TotalBytes   int64 `json:"totalBytes"`
	// BytesPerSecond is the current transfer rate.
	BytesPerSecond int64 `json:"bytesPerSecond"`
	// ErrorCode is a stable error identifier set when the operation fails.
	ErrorCode *string `json:"errorCode,omitempty"`
	// ErrorMessage is the human-readable failure description set when the operation fails.
	ErrorMessage *string `json:"errorMessage,omitempty"`
	// CreatedAt, StartedAt, and FinishedAt are the operation lifecycle timestamps.
	CreatedAt  string  `json:"createdAt"`
	StartedAt  *string `json:"startedAt,omitempty"`
	FinishedAt *string `json:"finishedAt,omitempty"`
}

func operationDTO(operation operations.Operation) OperationDTO {
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

// PlaySessionDTO identifies a running or completed game process for session views.
type PlaySessionDTO struct {
	ID string `json:"id"`
	// InstanceID is the instance the session belongs to.
	InstanceID string `json:"instanceId"`
	// AccountID is the account used for the session, when one was selected.
	AccountID *string `json:"accountId,omitempty"`
	// VersionID is the game version that ran during the session.
	VersionID string `json:"versionId"`
	// StartedAt and EndedAt bound the session lifetime.
	StartedAt string  `json:"startedAt"`
	EndedAt   *string `json:"endedAt,omitempty"`
	// DurationSeconds is the session length.
	DurationSeconds int64 `json:"durationSeconds"`
	// ExitCode is the process exit code, when the session has ended.
	ExitCode *int `json:"exitCode,omitempty"`
	// Crashed reports whether the process terminated unexpectedly.
	Crashed bool `json:"crashed"`
	// Recovered reports whether a last-known-good recovery was applied.
	Recovered bool `json:"recovered"`
}

func sessionDTO(session sessions.PlaySession) PlaySessionDTO {
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

// StatisticsDTO aggregates library and play-time metrics for the overview.
type StatisticsDTO struct {
	TotalPlaytimeSeconds  int64            `json:"totalPlaytimeSeconds"`
	LaunchCount           int              `json:"launchCount"`
	AverageSessionSeconds int64            `json:"averageSessionSeconds"`
	MostPlayedInstanceID  *string          `json:"mostPlayedInstanceId,omitempty"`
	RecentSessions        []PlaySessionDTO `json:"recentSessions"`
}

func statisticsDTO(statistics statistics.Statistics) StatisticsDTO {
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

// SettingsDTO carries launcher preferences between the frontend and settings service.
type SettingsDTO struct {
	Language                   string   `json:"language"`
	DownloadsParallel          int      `json:"downloadsParallel"`
	ConfirmDeletion            bool     `json:"confirmDeletion"`
	GlobalLaunchArguments      []string `json:"globalLaunchArguments"`
	OptimumPath                string   `json:"optimumPath"`
	CheckForUpdates            bool     `json:"checkForUpdates"`
	UpdateChannel              string   `json:"updateChannel"`
	SkippedUpdateVersion       string   `json:"skippedUpdateVersion"`
	TelemetryEnabled           bool     `json:"telemetryEnabled"`
	AutomaticSafetySnapshots   bool     `json:"automaticSafetySnapshots"`
	AutomaticSnapshotRetention int      `json:"automaticSnapshotRetention"`
	LibrarySort                string   `json:"librarySort"`
	UIScale                    float64  `json:"uiScale"`
}

func settingsDTO(settings settings.Settings) SettingsDTO {
	launchArguments := settings.GlobalLaunchArguments
	if launchArguments == nil {
		launchArguments = []string{}
	}

	return SettingsDTO{
		Language:                   settings.Language,
		DownloadsParallel:          settings.DownloadsParallel,
		ConfirmDeletion:            settings.ConfirmDeletion,
		GlobalLaunchArguments:      launchArguments,
		OptimumPath:                settings.OptimumPath,
		CheckForUpdates:            settings.CheckForUpdates,
		UpdateChannel:              settings.UpdateChannel,
		SkippedUpdateVersion:       settings.SkippedUpdateVersion,
		TelemetryEnabled:           settings.TelemetryEnabled,
		AutomaticSafetySnapshots:   settings.AutomaticSafetySnapshots,
		AutomaticSnapshotRetention: settings.AutomaticSnapshotRetention,
		LibrarySort:                settings.LibrarySort,
		UIScale:                    settings.UIScale,
	}
}

// DataFolderDTO reports the active launcher data path and storage usage.
type DataFolderDTO struct {
	CurrentPath string `json:"currentPath"`
	DefaultPath string `json:"defaultPath"`
	LastError   string `json:"lastError"`
}

// OptimumStatusDTO reports whether a usable Optimum installation was found.
type OptimumStatusDTO struct {
	Path        string `json:"path"`
	Executable  string `json:"executable"`
	GameVersion string `json:"gameVersion"`
	Ready       bool   `json:"ready"`
	Message     string `json:"message"`
}

func optimumStatusDTO(status optimum.Status) OptimumStatusDTO {
	return OptimumStatusDTO{
		Path: status.Path, Executable: status.Executable, GameVersion: status.GameVersion,
		Ready: status.Ready, Message: status.Message,
	}
}

// DataFolderProgressDTO reports progress while relocating launcher data.
type DataFolderProgressDTO struct {
	CopiedBytes int64   `json:"copiedBytes"`
	TotalBytes  int64   `json:"totalBytes"`
	Progress    float64 `json:"progress"`
	Phase       string  `json:"phase"`
}
