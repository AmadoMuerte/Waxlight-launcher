package waxlight

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"strings"
)

// buildVersion is injected by release builds with:
//
//	-X github.com/waxlight/waxlight-launcher.buildVersion=<semantic version>
//
// Windows requires a numeric four-part VERSIONINFO value, so the semantic
// launcher version must not be read from the temporarily rewritten wails.json.
var buildVersion string

var semanticVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

//go:embed wails.json
var applicationConfigJSON []byte

func Version() string {
	if version := canonicalSemanticVersion(buildVersion); version != "" {
		return version
	}

	var config struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(applicationConfigJSON, &config); err != nil {
		return "0.0.0"
	}

	version := normalizeVersion(config.Info.ProductVersion)
	if canonicalSemanticVersion(version) == "" {
		return "0.0.0"
	}
	return version
}

func canonicalSemanticVersion(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	if version == "" || !semanticVersionPattern.MatchString(version) {
		return ""
	}
	return version
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	parts := strings.SplitN(version, ".", 4)
	if len(parts) == 4 && parts[3] == "0" {
		return strings.Join(parts[:3], ".")
	}
	return version
}
