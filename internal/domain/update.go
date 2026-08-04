package domain

type LauncherUpdate struct {
	InstalledVersion string
	Version          string
	Available        bool
	Prerelease       bool
	ReleaseNotes     string
	ReleasePageURL   string
	AssetName        string
	AssetSize        int64
	DownloadURL      string
	SHA256           string
}

type LauncherUpdateProgress struct {
	Phase           string  `json:"phase"`
	DownloadedBytes int64   `json:"downloadedBytes"`
	TotalBytes      int64   `json:"totalBytes"`
	Progress        float64 `json:"progress"`
}
