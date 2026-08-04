package main

import (
	_ "embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	frontendassets "github.com/waxlight/waxlight-launcher/frontend"
	"github.com/waxlight/waxlight-launcher/internal/bootstrap"
)

// appIcon is embedded into the executable so Linux window managers can use
// the same icon as the packaged desktop entry.
//
//go:embed appicon.png
var appIcon []byte

func main() {
	container, err := bootstrap.New()
	if err != nil {
		log.Fatal(err)
	}

	err = wails.Run(&options.App{
		Title:            "Waxlight Launcher",
		Width:            1240,
		Height:           780,
		MinWidth:         620,
		MinHeight:        640,
		AssetServer:      &assetserver.Options{Assets: frontendassets.Assets},
		BackgroundColour: &options.RGBA{R: 13, G: 13, B: 16, A: 1},
		OnStartup:        container.Startup,
		OnShutdown:       container.Shutdown,
		Bind:             container.Controllers,
		Linux: &linux.Options{
			Icon:        appIcon,
			ProgramName: "waxlight",
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
