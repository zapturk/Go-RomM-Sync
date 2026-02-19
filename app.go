package main

import (
	"context"
	"fmt"
	"go-romm-sync/config"
	"go-romm-sync/romm"
	"go-romm-sync/types"
)

// App struct
type App struct {
	ctx           context.Context
	configManager *config.ConfigManager
	rommClient    *romm.Client
}

// NewApp creates a new App application struct
func NewApp(cm *config.ConfigManager) *App {
	cfg := cm.GetConfig()
	client := romm.NewClient(cfg.RommHost)
	return &App{
		configManager: cm,
		rommClient:    client,
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

	// Update client in case host changed
	a.rommClient = romm.NewClient(cfg.RommHost)

	return "Configuration saved successfully!"
}

// Login authenticates with the RomM server
func (a *App) Login() (string, error) {
	cfg := a.configManager.GetConfig()
	if cfg.RommHost == "" || cfg.Username == "" || cfg.Password == "" {
		return "", fmt.Errorf("missing configuration: host, username, or password")
	}

	// Ensure client is up to date with current config
	if a.rommClient.BaseURL == "" || a.rommClient.BaseURL != cfg.RommHost {
		a.rommClient = romm.NewClient(cfg.RommHost)
	}

	token, err := a.rommClient.Login(cfg.Username, cfg.Password)
	if err != nil {
		return "", err
	}
	return token, nil
}

// GetLibrary fetches the game library
func (a *App) GetLibrary() ([]types.Game, error) {
	return a.rommClient.GetLibrary()
}
