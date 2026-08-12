package updater

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type ReleaseManifest struct {
	SchemaVersion int                  `json:"schemaVersion"`
	Version       string               `json:"version"`
	Channel       string               `json:"channel"`
	PublishedAt   string               `json:"publishedAt"`
	Assets        map[string]AssetInfo `json:"assets"`
}

type AssetInfo struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	Publisher string `json:"publisher,omitempty"`
}

func GenerateManifest(version, channel, publishTime string, assets map[string]AssetInfo) *ReleaseManifest {
	if publishTime == "" {
		publishTime = time.Now().UTC().Format(time.RFC3339)
	}
	return &ReleaseManifest{
		SchemaVersion: 1,
		Version:       version,
		Channel:       channel,
		PublishedAt:   publishTime,
		Assets:        assets,
	}
}

func SaveManifest(manifest *ReleaseManifest, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create manifest directory: %w", err)
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	path := filepath.Join(outputDir, "update-manifest.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	return nil
}

func LoadManifest(path string) (*ReleaseManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var manifest ReleaseManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	if manifest.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported manifest schema version: %d", manifest.SchemaVersion)
	}

	return &manifest, nil
}
