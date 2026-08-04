package waxlight

import (
	_ "embed"
	"encoding/json"
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
	return config.Info.ProductVersion
}
