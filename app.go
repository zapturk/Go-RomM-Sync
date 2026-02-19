package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"go-romm-sync/config"
	"go-romm-sync/romm"
	"go-romm-sync/types"
	"os"
	"path/filepath"
	"strings"
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

// GetCover returns the base64 encoded cover image for a game
func (a *App) GetCover(romID uint, coverURL string) (string, error) {
	if coverURL == "" {
		return "", nil // No cover available
	}

	// Define cache directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home dir: %w", err)
	}
	cacheDir := filepath.Join(homeDir, ".go-romm-sync", "cache", "covers")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache dir: %w", err)
	}

	// Determine filename from romID (assuming jpg for simplicity, or we could hash the URL)
	// RomM seems to use jpg/png. Let's try to preserve extension or just default to jpg.
	// Check extension from URL
	ext := filepath.Ext(coverURL)
	if ext == "" {
		ext = ".jpg"
	}
	filename := fmt.Sprintf("%d%s", romID, ext)
	cachePath := filepath.Join(cacheDir, filename)

	// Check if file exists
	if _, err := os.Stat(cachePath); err == nil {
		// File exists, read and return base64
		data, err := os.ReadFile(cachePath)
		if err != nil {
			return "", fmt.Errorf("failed to read cached cover: %w", err)
		}
		return base64.StdEncoding.EncodeToString(data), nil
	}

	// File doesn't exist, download it
	data, err := a.rommClient.DownloadCover(coverURL)
	if err != nil {
		return "", fmt.Errorf("failed to download cover: %w", err)
	}

	// Save to cache
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		fmt.Printf("Warning: failed to write to cache: %v\n", err)
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

// GetPlatforms fetches the list of platforms
func (a *App) GetPlatforms() ([]types.Platform, error) {
	return a.rommClient.GetPlatforms()
}

// GetPlatformCover returns the data URI for the platform cover (e.g. data:image/svg+xml;base64,...)
func (a *App) GetPlatformCover(platformID uint, slug string) (string, error) {
	if slug == "" {
		return "", nil // No slug available
	}

	// Define cache directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home dir: %w", err)
	}
	cacheDir := filepath.Join(homeDir, ".go-romm-sync", "cache", "platforms")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache dir: %w", err)
	}

	extensions := []string{".svg", ".ico", ".png", ".jpg"}

	// Check cache for any existing file
	for _, ext := range extensions {
		filename := fmt.Sprintf("%d%s", platformID, ext)
		cachePath := filepath.Join(cacheDir, filename)
		if _, err := os.Stat(cachePath); err == nil {
			data, err := os.ReadFile(cachePath)
			if err != nil {
				continue
			}
			mimeType := getMimeType(ext)
			return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data)), nil
		}
	}

	// Not in cache, try downloading
	var data []byte
	var foundExt string

	// Try original slug with different extensions
	for _, ext := range extensions {
		url := fmt.Sprintf("/assets/platforms/%s%s", slug, ext)
		d, err := a.rommClient.DownloadCover(url)
		if err == nil {
			data = d
			foundExt = ext
			break
		}
	}

	if data == nil {
		// Fallback: Try replacing hyphens with underscores
		if strings.Contains(slug, "-") {
			altSlug := strings.ReplaceAll(slug, "-", "_")
			for _, ext := range extensions {
				url := fmt.Sprintf("/assets/platforms/%s%s", altSlug, ext)
				d, err := a.rommClient.DownloadCover(url)
				if err == nil {
					data = d
					foundExt = ext
					break
				}
			}
		}
	}

	if data == nil {
		return "", fmt.Errorf("failed to download cover")
	}

	// Save to cache
	filename := fmt.Sprintf("%d%s", platformID, foundExt)
	cachePath := filepath.Join(cacheDir, filename)
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		fmt.Printf("Warning: failed to write to cache: %v\n", err)
	}

	mimeType := getMimeType(foundExt)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data)), nil
}

func getMimeType(ext string) string {
	switch ext {
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	default:
		return "application/octet-stream"
	}
}
