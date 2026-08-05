package waxlight

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed wails.json
var applicationConfigJSON []byte

func Version() string {
	var config struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(applicationConfigJSON, &config); err != nil || config.Info.ProductVersion == "" {
		return "0.0.0"
	}
	return normalizeVersion(config.Info.ProductVersion)
}

func normalizeVersion(version string) string {
	parts := strings.SplitN(version, ".", 4)
	if len(parts) == 4 && parts[3] == "0" {
		return strings.Join(parts[:3], ".")
	}
	return version
}
