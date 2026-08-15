package main

import (
	"context"
	_ "embed"
	"log/slog"
	"os"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	frontendassets "github.com/waxlight/waxlight-launcher/frontend"
	"github.com/waxlight/waxlight-launcher/internal/app"
	"github.com/waxlight/waxlight-launcher/internal/platform/deeplink"
	"github.com/waxlight/waxlight-launcher/internal/platform/logging"
	"github.com/waxlight/waxlight-launcher/internal/platform/mousenavigation"
	"github.com/waxlight/waxlight-launcher/internal/platform/updater"
)

// appIcon is embedded into the executable so Linux window managers can use
// the same icon as the packaged desktop entry.
//
//go:embed appicon.png
var appIcon []byte

func main() {
	// Set up the shared launcher logger before anything else so every log
	// line, including construction failures, reaches the in-memory console.
	logging.Setup(logging.DefaultCapacity)
	if err := deeplink.RegisterHandler(); err != nil {
		slog.Warn("Failed to register Waxlight links", "error", err)
	}

	// The updated portable binary is launched by the old process with
	// --update-wait-pid <oldpid> before the old process exits. Wait for it to
	// release the database and the native credential store before bootstrapping
	// this instance. WaitForParent is a no-op on Windows.
	if pid := updateWaitPID(); pid > 0 {
		updater.WaitForParent(pid, 30*time.Second)
	}

	container, err := app.New()
	if err != nil {
		logging.Fatal("Failed to construct the launcher", err)
	}
	container.DeepLinks.ReceiveArgs(os.Args[1:])

	err = wails.Run(&options.App{
		Title:            "Waxlight Launcher",
		Width:            1240,
		Height:           780,
		MinWidth:         620,
		MinHeight:        640,
		AssetServer:      &assetserver.Options{Assets: frontendassets.Assets, Handler: container.CoverHandler},
		BackgroundColour: &options.RGBA{R: 13, G: 13, B: 16, A: 1},
		OnStartup:        container.Startup,
		OnDomReady: func(ctx context.Context) {
			mousenavigation.Install(func(direction int) {
				container.Events.Publish("navigation:mouse", direction)
			})
		},
		OnShutdown: container.Shutdown,
		Bind:       container.Controllers,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "1b6bb7a2-2ecb-4a76-85b1-09b1b4c6c0fd",
			OnSecondInstanceLaunch: func(data options.SecondInstanceData) {
				container.DeepLinks.ReceiveArgs(data.Args)
				wruntime.WindowUnminimise(container.Lifecycle.Context())
				wruntime.Show(container.Lifecycle.Context())
			},
		},
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
		logging.Fatal("The launcher window failed to start", err)
	}
}
