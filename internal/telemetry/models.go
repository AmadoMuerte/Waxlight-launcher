package telemetry

// Heartbeat is the payload sent to POST /v1/heartbeat. It carries only
// numeric counts and system identifiers that describe the launcher
// installation, never personal data, paths, names, or account information.
type Heartbeat struct {
	InstallationID string `json:"installation_id"`
	AppVersion     string `json:"app_version"`
	OS             string `json:"os"`
	Arch           string `json:"arch"`
	InstancesCount int    `json:"instances_count"`
	ModsCount      int    `json:"mods_count"`
}

// Event is the payload sent to POST /v1/events. It describes one allowlisted
// lifecycle operation by name only.
type Event struct {
	InstallationID string `json:"installation_id"`
	AppVersion     string `json:"app_version"`
	OS             string `json:"os"`
	Arch           string `json:"arch"`
	Event          string `json:"event"`
}

// ErrorEvent is the payload sent to POST /v1/errors. It carries only
// allowlisted structured error categories. Raw error text, stack traces,
// paths, and response bodies never enter this payload.
type ErrorEvent struct {
	InstallationID string `json:"installation_id"`
	AppVersion     string `json:"app_version"`
	OS             string `json:"os"`
	Arch           string `json:"arch"`
	ErrorCode      string `json:"error_code"`
	Component      string `json:"component"`
	Operation      string `json:"operation"`
}

type SupportReportResult struct {
	ReportID string `json:"reportId"`
	Status   string `json:"status"`
}
