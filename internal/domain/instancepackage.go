package domain

import "time"

const InstancePackageSchemaVersion = 1

type PackageModSource string

const (
	PackageModSourceCatalog  PackageModSource = "moddb"
	PackageModSourceEmbedded PackageModSource = "embedded"
)

// PackageAuthor carries optional attribution for a shared package.
type PackageAuthor struct {
	Name     string `json:"name,omitempty"`
	Homepage string `json:"homepage,omitempty"`
	Source   string `json:"source,omitempty"`
}

// PackageGameVersion identifies the Vintage Story release required by an
// instance. ID is the launcher version identifier and Name is the human
// readable catalog name; both are preserved so import can match an installed
// version even when identifiers differ between machines.
type PackageGameVersion struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PackageMod describes one installed mod in a package manifest. Catalog mods
// carry a ModID/VersionID reference and are re-downloaded on import. Mods that
// cannot be resolved (or were installed from a local file) are embedded in the
// package and referenced by ArchivePath.
type PackageMod struct {
	ModID       string           `json:"modId,omitempty"`
	VersionID   string           `json:"versionId,omitempty"`
	Name        string           `json:"name"`
	Version     string           `json:"version,omitempty"`
	FileName    string           `json:"fileName"`
	Source      PackageModSource `json:"source"`
	Checksum    string           `json:"checksum,omitempty"`
	DownloadURL string           `json:"downloadUrl,omitempty"`
	Enabled     bool             `json:"enabled"`
	ArchivePath string           `json:"-"`
}

// PackageManifest is the versioned, portable description of an instance.
type PackageManifest struct {
	SchemaVersion   int                `json:"schemaVersion"`
	Name            string             `json:"name"`
	Description     string             `json:"description,omitempty"`
	Author          *PackageAuthor     `json:"author,omitempty"`
	GameVersion     PackageGameVersion `json:"gameVersion"`
	LaunchArguments []string           `json:"launchArguments,omitempty"`
	Mods            []PackageMod       `json:"mods,omitempty"`
	ConfigFiles     []string           `json:"configFiles,omitempty"`
	HasIcon         bool               `json:"hasIcon,omitempty"`
	CreatedAt       time.Time          `json:"createdAt,omitempty"`
}

// ConfigFileSet returns the declared config paths as a lookup set.
func (m PackageManifest) ConfigFileSet() map[string]struct{} {
	result := make(map[string]struct{}, len(m.ConfigFiles))
	for _, relative := range m.ConfigFiles {
		result[relative] = struct{}{}
	}
	return result
}

type PackageModStatus string

const (
	PackageModAvailable    PackageModStatus = "available"
	PackageModIncompatible PackageModStatus = "incompatible"
	PackageModMissing      PackageModStatus = "missing"
	PackageModEmbedded     PackageModStatus = "embedded"
)

// PackageModCheck reports how a mod from a package would be handled on import.
type PackageModCheck struct {
	ModID       string           `json:"modId,omitempty"`
	VersionID   string           `json:"versionId,omitempty"`
	Name        string           `json:"name"`
	Version     string           `json:"version"`
	Source      PackageModSource `json:"source"`
	Enabled     bool             `json:"enabled"`
	Status      PackageModStatus `json:"status"`
	Message     string           `json:"message,omitempty"`
	HasEmbedded bool             `json:"hasEmbedded,omitempty"`
}

type PackageVersionStatus string

const (
	PackageVersionInstalled PackageVersionStatus = "installed"
	PackageVersionAvailable PackageVersionStatus = "available"
	PackageVersionMissing   PackageVersionStatus = "missing"
)

// PackageInspection is the result of validating a package before import.
type PackageInspection struct {
	Path            string
	SchemaVersion   int
	Name            string
	Description     string
	Author          *PackageAuthor
	GameVersion     PackageGameVersion
	VersionStatus   PackageVersionStatus
	LaunchArguments []string
	Mods            []PackageModCheck
	ConfigFiles     []string
	HasIcon         bool
	TotalSize       int64
	UnverifiedFiles int
	Warnings        []string
}

// ExportInstanceOptions controls how an instance is packaged.
type ExportInstanceOptions struct {
	Name        string
	Description string
	Author      *PackageAuthor
}

// ImportInstanceOptions controls how a package is installed as a new instance.
type ImportInstanceOptions struct {
	Name              string
	Description       string
	Directory         string
	GameVersionID     string
	InstallVersion    bool
	AllowIncompatible bool
	SkipUnavailable   bool
}

type ImportedModResult struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// ImportReport summarizes what was installed when importing a package.
type ImportReport struct {
	InstanceID    string              `json:"instanceId"`
	InstanceName  string              `json:"instanceName"`
	GameVersionID string              `json:"gameVersionId"`
	Mods          []ImportedModResult `json:"mods"`
	Warnings      []string            `json:"warnings"`
}
