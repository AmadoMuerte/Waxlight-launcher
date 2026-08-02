package frontend

import "embed"

// Assets contains the production React bundle.
//
//go:embed all:dist
var Assets embed.FS
