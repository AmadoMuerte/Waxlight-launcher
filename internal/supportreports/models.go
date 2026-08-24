package supportreports

import "time"

const (
	SchemaVersion        = 1
	MaxDescriptionLength = 2000
	MaxOperations        = 20
	MaxMods              = 1000
	MaxLaunchArguments   = 100
	MaxEnvironment       = 100
	MaxLogLines          = 300
	MaxLogBytes          = 64 * 1024
	MaxPayloadBytes      = 256 * 1024
)

type Launcher struct {
	Version  string `json:"version"`
	Commit   string `json:"commit,omitempty"`
	Platform string `json:"platform"`
	Arch     string `json:"arch"`
}

type System struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	GoVersion string `json:"goVersion"`
}

type Instance struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name,omitempty"`
	GameVersion          string            `json:"gameVersion"`
	Client               string            `json:"client"`
	Status               string            `json:"status"`
	LaunchArguments      []string          `json:"launchArguments,omitempty"`
	EnvironmentVariables map[string]string `json:"environmentVariables,omitempty"`
	ModCount             int               `json:"modCount"`
	EnabledModCount      int               `json:"enabledModCount"`
}

type Mod struct {
	ModID        string `json:"modId"`
	Version      string `json:"version"`
	Enabled      bool   `json:"enabled"`
	Source       string `json:"source"`
	UpdatePolicy string `json:"updatePolicy"`
}

type Operation struct {
	Type         string     `json:"type"`
	Status       string     `json:"status"`
	Progress     float64    `json:"progress,omitempty"`
	CurrentBytes int64      `json:"currentBytes,omitempty"`
	TotalBytes   int64      `json:"totalBytes,omitempty"`
	ErrorCode    string     `json:"errorCode,omitempty"`
	Error        string     `json:"error,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
}

type Launch struct {
	GameVersion   string     `json:"gameVersion"`
	StartedAt     time.Time  `json:"startedAt"`
	EndedAt       *time.Time `json:"endedAt,omitempty"`
	DurationSec   int64      `json:"durationSec,omitempty"`
	ExitCode      *int       `json:"exitCode,omitempty"`
	StartupFailed bool       `json:"startupFailed"`
}

type Recovery struct {
	LastKnownGoodExists bool `json:"lastKnownGoodExists"`
	SnapshotCount       int  `json:"snapshotCount"`
}

type Logs struct {
	Launcher []string `json:"launcher,omitempty"`
}

type Report struct {
	SchemaVersion  int         `json:"schemaVersion"`
	InstallationID string      `json:"installationId"`
	Description    string      `json:"description"`
	Launcher       Launcher    `json:"launcher"`
	System         System      `json:"system"`
	Instance       *Instance   `json:"instance,omitempty"`
	Mods           []Mod       `json:"mods"`
	Operations     []Operation `json:"operations"`
	Launch         *Launch     `json:"launch,omitempty"`
	Recovery       *Recovery   `json:"recovery,omitempty"`
	Logs           Logs        `json:"logs"`
}

type Preview struct {
	SnapshotID string `json:"snapshotId"`
	Payload    string `json:"payload"`
}

type Result struct {
	ReportID string `json:"reportId"`
	Status   string `json:"status"`
}
