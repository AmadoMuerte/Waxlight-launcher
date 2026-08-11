// Package settings owns launcher preferences and data-root orchestration.
package settings

// Settings contains user-configurable launcher preferences.
type Settings struct {
	Language                 string
	DownloadsParallel        int
	ConfirmDeletion          bool
	GlobalLaunchArguments    []string
	CheckForUpdates          bool
	UpdateChannel            string
	SkippedUpdateVersion     string
	TelemetryEnabled         bool
	AutomaticSafetySnapshots bool
}

func Defaults() Settings {
	return Settings{
		Language:                 "en",
		DownloadsParallel:        3,
		ConfirmDeletion:          true,
		GlobalLaunchArguments:    []string{},
		CheckForUpdates:          true,
		UpdateChannel:            "stable",
		TelemetryEnabled:         false,
		AutomaticSafetySnapshots: true,
	}
}

type DataFolder struct {
	CurrentPath string `json:"currentPath"`
	DefaultPath string `json:"defaultPath"`
	LastError   string `json:"lastError"`
}

type RelocationProgress struct {
	CopiedBytes int64   `json:"copiedBytes"`
	TotalBytes  int64   `json:"totalBytes"`
	Progress    float64 `json:"progress"`
	Phase       string  `json:"phase"`
}
