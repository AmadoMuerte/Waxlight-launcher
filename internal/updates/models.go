// Package updates owns launcher update checks, verified downloads,
// installation orchestration, and update progress.
package updates

// Update describes a launcher release discovered by the update source.
type Update struct {
	InstalledVersion string
	Version          string
	Available        bool
	Downgrade        bool
	Prerelease       bool
	ReleaseNotes     string
	ReleasePageURL   string
	AssetName        string
	AssetSize        int64
	DownloadURL      string
	SHA256           string
	InstallationMode string
}

// Progress is a JSON-friendly update progress report forwarded to the UI
// through the updates:progress event.
type Progress struct {
	Phase           string  `json:"phase"`
	DownloadedBytes int64   `json:"downloadedBytes"`
	TotalBytes      int64   `json:"totalBytes"`
	Progress        float64 `json:"progress"`
}

// Stage names an update lifecycle phase. The values are part of the
// frontend contract (update_phase_* translation keys) and must not change.
type Stage string

const (
	StageChecking    Stage = "checking"
	StageDownloading Stage = "downloading"
	StageSignature   Stage = "signature"
	StageInstalling  Stage = "installing"
	StageRestarting  Stage = "restarting"
)
