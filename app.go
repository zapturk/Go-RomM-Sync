package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"go-romm-sync/config"
	"go-romm-sync/retroarch"
	"go-romm-sync/romm"
	"go-romm-sync/types"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
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

// Quit closes the application
func (a *App) Quit() {
	wailsRuntime.Quit(a.ctx)
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
	current := a.configManager.GetConfig()
	oldHost := current.RommHost

	// Update fields if provided
	updateIfNotEmpty(&current.RommHost, cfg.RommHost)
	updateIfNotEmpty(&current.Username, cfg.Username)
	updateIfNotEmpty(&current.Password, cfg.Password)
	updateIfNotEmpty(&current.LibraryPath, cfg.LibraryPath)
	updateIfNotEmpty(&current.RetroArchPath, cfg.RetroArchPath)
	updateIfNotEmpty(&current.RetroArchExecutable, cfg.RetroArchExecutable)
	updateIfNotEmpty(&current.CheevosUsername, cfg.CheevosUsername)
	updateIfNotEmpty(&current.CheevosPassword, cfg.CheevosPassword)

	if err := a.configManager.Save(current); err != nil {
		return fmt.Sprintf("Error saving config: %s", err.Error())
	}

	// Update client only if host changed to preserve session token
	if current.RommHost != oldHost {
		a.rommClient = romm.NewClient(current.RommHost)
	}

	return "Configuration saved successfully!"
}

// updateIfNotEmpty is a helper to only update a field if the new value is not empty
func updateIfNotEmpty(target *string, value string) {
	if value != "" {
		*target = value
	}
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

// GetRom fetches a single ROM from RomM
func (a *App) GetRom(id uint) (types.Game, error) {
	return a.rommClient.GetRom(id)
}

// DownloadRom returns the download URL for a ROM
func (a *App) DownloadRom(id uint) (string, error) {
	cfg := a.configManager.GetConfig()
	if cfg.RommHost == "" {
		return "", fmt.Errorf("missing RomM host configuration")
	}
	// RomM download URL structure: {host}/api/roms/{id}/download
	downloadURL := fmt.Sprintf("%s/api/roms/%d/download", strings.TrimRight(cfg.RommHost, "/"), id)
	return downloadURL, nil
}

// getRomDir returns the local directory where a ROM is stored
func (a *App) getRomDir(game *types.Game) string {
	cfg := a.configManager.GetConfig()
	return filepath.Join(cfg.LibraryPath, filepath.Dir(game.FullPath), fmt.Sprintf("%d", game.ID))
}

// DownloadRomToLibrary downloads a ROM directly to the configured library path
func (a *App) DownloadRomToLibrary(id uint) error {
	cfg := a.configManager.GetConfig()
	if cfg.LibraryPath == "" {
		defaultPath, err := config.GetDefaultLibraryPath()
		if err != nil {
			return fmt.Errorf("library path is not configured and failed to determine default: %w", err)
		}
		cfg.LibraryPath = defaultPath
		// Save the default path so the user doesn't hit this again
		if err := a.configManager.Save(cfg); err != nil {
			fmt.Printf("Warning: failed to save default library path: %v\n", err)
		}
	}

	// 1. Get ROM info to know where it belongs
	game, err := a.rommClient.GetRom(id)
	if err != nil {
		return fmt.Errorf("failed to get ROM info: %w", err)
	}

	// 2. Start download
	reader, filename, err := a.rommClient.DownloadFile(&game)
	if err != nil {
		return err
	}
	defer reader.Close()

	if filename == "" {
		// Fallback to a name derived from game title if header missing
		filename = game.Title
	}

	// 3. Determine destination
	destDir := a.getRomDir(&game)
	filename = filepath.Base(game.FullPath)
	destPath := filepath.Join(destDir, filename)

	// 4. Create directory and save file
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, reader)
	if err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}

// GetRomDownloadStatus checks if a ROM has been downloaded to the library
func (a *App) GetRomDownloadStatus(id uint) (bool, error) {
	cfg := a.configManager.GetConfig()
	if cfg.LibraryPath == "" {
		return false, nil
	}

	game, err := a.rommClient.GetRom(id)
	if err != nil {
		return false, nil // If we can't find the ROM info, assume not downloaded
	}

	romDir := a.getRomDir(&game)

	if info, err := os.Stat(romDir); err == nil && info.IsDir() {
		return a.findRomPath(romDir) != "", nil
	}

	return false, nil
}

// findRomPath looks for a valid ROM file in the given directory
func (a *App) findRomPath(romDir string) string {
	files, err := os.ReadDir(romDir)
	if err != nil {
		return ""
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if strings.HasPrefix(name, ".") {
			continue // Skip hidden files like .DS_Store
		}
		ext := strings.ToLower(filepath.Ext(name))
		if _, ok := retroarch.CoreMap[ext]; ok || ext == ".zip" {
			return filepath.Join(romDir, name)
		}
	}
	return ""
}

// DeleteRom removes a downloaded ROM from the local library
func (a *App) DeleteRom(id uint) error {
	cfg := a.configManager.GetConfig()
	if cfg.LibraryPath == "" {
		return fmt.Errorf("library path is not configured")
	}

	game, err := a.rommClient.GetRom(id)
	if err != nil {
		return fmt.Errorf("failed to get ROM info for deletion: %w", err)
	}

	romDir := a.getRomDir(&game)

	if _, err := os.Stat(romDir); err == nil {
		if err := os.RemoveAll(romDir); err != nil {
			wailsRuntime.LogErrorf(a.ctx, "DeleteRom: Error during RemoveAll for ID %d: %v", id, err)
			return fmt.Errorf("failed to delete ROM directory: %w", err)
		}
		wailsRuntime.LogInfof(a.ctx, "DeleteRom: Successfully deleted ROM %d from library", id)
	}

	return nil
}

// PlayRom attempts to launch the given ROM with RetroArch
func (a *App) PlayRom(id uint) error {
	cfg := a.configManager.GetConfig()
	if cfg.LibraryPath == "" {
		return fmt.Errorf("library path is not configured")
	}

	// 1. Get ROM info
	game, err := a.rommClient.GetRom(id)
	if err != nil {
		return fmt.Errorf("failed to get ROM info: %w", err)
	}

	// 2. Find local ROM path
	romDir := a.getRomDir(&game)
	romPath := a.findRomPath(romDir)
	if romPath == "" {
		return fmt.Errorf("no valid ROM file found in %s. Please download it first.", romDir)
	}

	// 3. Check if RetroArch is Configured
	exePath := cfg.RetroArchPath
	if exePath == "" {
		// Prompt user manually if they haven't set it yet
		exePath, err = a.SelectRetroArchExecutable()
		if err != nil {
			return fmt.Errorf("retroarch not configured: %w", err)
		}
		if exePath == "" {
			return fmt.Errorf("launch cancelled: RetroArch executable not selected")
		}
	} else {
		// Verify the configured path exists
		if _, err := os.Stat(exePath); err != nil {
			return fmt.Errorf("retroarch executable not found at configured path: %s", exePath)
		}
	}

	// 4. Launch the game
	err = retroarch.Launch(a.ctx, exePath, romPath, cfg.CheevosUsername, cfg.CheevosPassword)
	if err != nil {
		return fmt.Errorf("failed to launch game: %w", err)
	}

	return nil
}

// SelectRetroArchExecutable opens a file dialog for the user to select the RetroArch executable.
func (a *App) SelectRetroArchExecutable() (string, error) {
	options := wailsRuntime.OpenDialogOptions{
		Title: "Select RetroArch Executable",
		Filters: []wailsRuntime.FileFilter{
			{
				DisplayName: "All Files",
				Pattern:     "*.*",
			},
			{
				DisplayName: "Executables",
				Pattern:     "*.exe;*.app;retroarch",
			},
		},
	}

	if runtime.GOOS == "darwin" {
		options.DefaultDirectory = "/Applications"
		options.TreatPackagesAsDirectories = false // Allow selecting the .app bundle as a file
		options.Filters = nil                      // Remove all filters on macOS
	}

	selectedFile, err := wailsRuntime.OpenFileDialog(a.ctx, options)
	if err != nil {
		return "", err
	}

	if selectedFile != "" {
		// Save to config
		cfg := a.configManager.GetConfig()
		cfg.RetroArchPath = selectedFile
		err = a.configManager.Save(cfg)
		if err != nil {
			return "", fmt.Errorf("failed to save config: %w", err)
		}
	}

	return selectedFile, nil
}
