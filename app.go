package main

import (
	"context"
	"fmt"
	"go-romm-sync/config"
	"go-romm-sync/types"
)

// App struct
type App struct {
	ctx           context.Context
	configManager *config.ConfigManager
}

// NewApp creates a new App application struct
func NewApp(cm *config.ConfigManager) *App {
	return &App{
		configManager: cm,
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// GetConfig returns the current configuration
func (a *App) GetConfig() types.AppConfig {
	return a.configManager.GetConfig()
}

// SaveConfig saves the configuration
func (a *App) SaveConfig(cfg types.AppConfig) string {
	// Preserve existing paths if they are empty in the incoming config (simple merge strategy)
	current := a.configManager.GetConfig()
	if cfg.LibraryPath == "" {
		cfg.LibraryPath = current.LibraryPath
	}
	if cfg.RetroArchPath == "" {
		cfg.RetroArchPath = current.RetroArchPath
	}
	if cfg.RetroArchExecutable == "" {
		cfg.RetroArchExecutable = current.RetroArchExecutable
	}

	err := a.configManager.Save(cfg)
	if err != nil {
		return fmt.Sprintf("Error saving config: %s", err.Error())
	}
	return "Configuration saved successfully!"
}
