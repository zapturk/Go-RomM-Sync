package main

import (
	"embed"

	"go-romm-sync/config"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var distAssets embed.FS

func main() {
	// Create an instance of the app structure
	cm := config.NewConfigManager()
	if err := cm.Load(); err != nil {
		println("Error loading config:", err.Error())
	}

	app := NewApp(cm)

	// Create application with options
	err := wails.Run(&options.App{
		Title:      "Go RomM Sync",
		Width:      1920,
		Height:     1080,
		Fullscreen: true,
		AssetServer: &assetserver.Options{
			Assets: distAssets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
