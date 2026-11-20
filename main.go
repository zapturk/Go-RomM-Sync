package main

import (
	"embed"
	"log"
	"net/http"
	"strings"

	"go-romm-sync/config"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var distAssets embed.FS

func main() {
	// Create an instance of the app structure
	cm := config.NewConfigManager()
	if err := cm.Load(); err != nil {
		println("Error loading config:", err.Error())
	}

	appInstance := NewApp(cm)

	wailsApp := application.New(application.Options{
		Name:        "Go RomM Sync",
		Description: "A seamless bridge between RomM and RetroArch for a console-like gaming experience.",
		Services: []application.Service{
			application.NewService(appInstance),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(distAssets),
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasPrefix(r.URL.Path, "/cache/") {
						appInstance.assetSrv.ServeHTTP(w, r)
						return
					}
					next.ServeHTTP(w, r)
				})
			},
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	appInstance.SetWailsApp(wailsApp)

	mainWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Go RomM Sync",
		Width:            1920,
		Height:           1080,
		StartState:       application.WindowStateFullscreen,
		BackgroundColour: application.NewRGB(27, 38, 54),
		URL:              "/",
		DevToolsEnabled:  true,
	})

	mainWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		wailsApp.Quit()
	})

	appInstance.SetMainWindow(mainWindow)

	err := wailsApp.Run()
	if err != nil {
		log.Fatal(err)
	}
}
