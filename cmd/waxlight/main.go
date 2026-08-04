package main

import (
	"os"
	"strconv"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	frontendassets "github.com/waxlight/waxlight-launcher/frontend"
	"github.com/waxlight/waxlight-launcher/internal/bootstrap"
	"github.com/waxlight/waxlight-launcher/internal/infrastructure/updater"
)

func main() {
	updater.WaitForParent(updateParentPID(os.Args), 15*time.Second)
	container, err := bootstrap.New()
	if err != nil {
		showFatalError(err.Error())
		return
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
		Linux:            &linux.Options{ProgramName: "waxlight"},
		Windows:          &windows.Options{WebviewIsTransparent: false, WindowIsTranslucent: false},
	})
	if err != nil {
		showFatalError(err.Error())
	}
}

func updateParentPID(arguments []string) int {
	for index := 1; index+1 < len(arguments); index++ {
		if arguments[index] != "--update-wait-pid" {
			continue
		}
		pid, err := strconv.Atoi(arguments[index+1])
		if err == nil && pid > 0 {
			return pid
		}
		return 0
	}
	return 0
}
