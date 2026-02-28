package main

import (
	"context"
	"fmt"
	"go-romm-sync/config"
	"go-romm-sync/configsrv"
	"go-romm-sync/launcher"
	"go-romm-sync/library"
	"go-romm-sync/retroarch"
	"go-romm-sync/rommsrv"
	"go-romm-sync/sync"
	"go-romm-sync/types"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"go-romm-sync/constants"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx           context.Context
	configManager *config.ConfigManager
	configSrv     *configsrv.Service
	rommSrv       *rommsrv.Service
	librarySrv    *library.Service
	syncSrv       *sync.Service
	launcher      *launcher.Launcher
}

// NewApp creates a new App application struct
func NewApp(cm *config.ConfigManager) *App {
	app := &App{
		configManager: cm,
	}
	app.configSrv = configsrv.New(app, app)
	app.rommSrv = rommsrv.New(app)
	app.librarySrv = library.New(app, app, app)
	app.syncSrv = sync.New(app, app, app)
	app.launcher = launcher.New(app, app, app)
	return app
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.launcher.SetContext(ctx)
}

// --- Wails External API (Thin Wrappers) ---

// Config
func (a *App) GetConfig() types.AppConfig {
	return a.configSrv.GetConfig()
}

func (a *App) SaveConfig(cfg *types.AppConfig) string {
	// Use atomic update to prevent race conditions
	var hostOrCredsChanged bool
	err := a.configManager.Update(func(current *types.AppConfig) {
		oldHost := current.RommHost
		oldUser := current.Username
		oldPass := current.Password

		updateIfNotEmpty(&current.RommHost, cfg.RommHost)
		updateIfNotEmpty(&current.Username, cfg.Username)
		updateIfNotEmpty(&current.Password, cfg.Password)
		updateIfNotEmpty(&current.LibraryPath, cfg.LibraryPath)
		updateIfNotEmpty(&current.RetroArchPath, cfg.RetroArchPath)
		updateIfNotEmpty(&current.RetroArchExecutable, cfg.RetroArchExecutable)
		updateIfNotEmpty(&current.CheevosUsername, cfg.CheevosUsername)
		updateIfNotEmpty(&current.CheevosPassword, cfg.CheevosPassword)

		if current.RommHost != oldHost || current.Username != oldUser || current.Password != oldPass {
			hostOrCredsChanged = true
		}
	})

	if err != nil {
		return fmt.Sprintf("Error saving config: %v", err)
	}

	// Re-initialize RomM service if host or credentials changed
	if hostOrCredsChanged {
		a.rommSrv = rommsrv.New(a)
	}

	// Clear RetroArch cheevos token on save to ensure fresh login on credentials change
	fullCfg := a.configManager.GetConfig()
	if fullCfg.RetroArchPath != "" {
		if err := retroarch.ClearCheevosToken(fullCfg.RetroArchPath); err != nil {
			a.LogErrorf("Failed to clear RetroArch cheevos token: %v", err)
		}
	}

	return "Configuration saved successfully!"
}

func updateIfNotEmpty(target *string, value string) {
	if value != "" {
		*target = value
	}
}

func (a *App) SelectRetroArchExecutable() (string, error) {
	return a.configSrv.SelectRetroArchExecutable()
}

func (a *App) SelectLibraryPath() (string, error) {
	return a.configSrv.SelectLibraryPath()
}

func (a *App) GetDefaultLibraryPath() (string, error) {
	return a.configSrv.GetDefaultLibraryPath()
}

// RomM
func (a *App) Login() (string, error) {
	return a.rommSrv.Login()
}

func (a *App) Logout() error {
	// 1. Atomic update to clear credentials
	if err := a.configManager.Update(func(cfg *types.AppConfig) {
		cfg.Username = ""
		cfg.Password = ""
		cfg.CheevosUsername = ""
		cfg.CheevosPassword = ""
	}); err != nil {
		return err
	}

	// 2. Clear RetroArch cheevos token
	fullCfg := a.configManager.GetConfig()
	if fullCfg.RetroArchPath != "" {
		if err := retroarch.ClearCheevosToken(fullCfg.RetroArchPath); err != nil {
			a.LogErrorf("Failed to clear RetroArch cheevos token: %v", err)
		}
	}

	// 3. Reset RomM service to clear in-memory session/token
	a.rommSrv = rommsrv.New(a)

	return nil
}

func (a *App) GetLibrary() ([]types.Game, error) {
	return a.rommSrv.GetLibrary()
}

func (a *App) GetPlatforms() ([]types.Platform, error) {
	return a.rommSrv.GetPlatforms()
}

func (a *App) DownloadRom(id uint) (string, error) {
	cfg := a.configManager.GetConfig()
	if cfg.RommHost == "" {
		return "", fmt.Errorf("missing RomM host configuration")
	}
	return fmt.Sprintf("%s/api/roms/%d/download", strings.TrimRight(cfg.RommHost, "/"), id), nil
}

func (a *App) GetCover(romID uint, coverURL string) (string, error) {
	return a.rommSrv.GetCover(romID, coverURL)
}

func (a *App) GetPlatformCover(platformID uint, slug string) (string, error) {
	return a.rommSrv.GetPlatformCover(platformID, slug)
}

func (a *App) GetServerSaves(id uint) ([]types.ServerSave, error) {
	return a.rommSrv.GetServerSaves(id)
}

func (a *App) GetServerStates(id uint) ([]types.ServerState, error) {
	return a.rommSrv.GetServerStates(id)
}

// Library
func (a *App) DownloadRomToLibrary(id uint) error {
	return a.librarySrv.DownloadRomToLibrary(id)
}

func (a *App) GetRomDownloadStatus(id uint) (bool, error) {
	return a.librarySrv.GetRomDownloadStatus(id)
}

func (a *App) DeleteRom(id uint) error {
	return a.librarySrv.DeleteRom(id)
}

// Sync
func (a *App) GetSaves(id uint) ([]types.FileItem, error) {
	return a.syncSrv.GetSaves(id)
}

func (a *App) GetStates(id uint) ([]types.FileItem, error) {
	return a.syncSrv.GetStates(id)
}

func (a *App) DeleteSave(id uint, core, filename string) error {
	return a.syncSrv.DeleteGameFile(id, constants.DirSaves, core, filename)
}

func (a *App) DeleteState(id uint, core, filename string) error {
	return a.syncSrv.DeleteGameFile(id, constants.DirStates, core, filename)
}

func (a *App) UploadSave(id uint, core, filename string) error {
	return a.syncSrv.UploadSave(id, core, filename)
}

func (a *App) UploadState(id uint, core, filename string) error {
	return a.syncSrv.UploadState(id, core, filename)
}

func (a *App) DownloadServerSave(gameID uint, filePath, core, filename, updatedAt string) error {
	return a.syncSrv.DownloadServerSave(gameID, filePath, core, filename, updatedAt)
}

func (a *App) DownloadServerState(gameID uint, filePath, core, filename, updatedAt string) error {
	return a.syncSrv.DownloadServerState(gameID, filePath, core, filename, updatedAt)
}

func (a *App) ValidateAssetPath(core, filename string) (coreBase, fileBase string, err error) {
	return a.syncSrv.ValidateAssetPath(core, filename)
}

// Launch
func (a *App) PlayRom(id uint) error {
	return a.launcher.PlayRom(id)
}

// PlayRomWithCore launches the ROM using the specified libretro core base name
// (e.g. "snes9x_libretro"). Allows the user to override the default core.
func (a *App) PlayRomWithCore(id uint, coreName string) error {
	return a.launcher.PlayRomWithCore(id, coreName)
}

// GetCoresForGame returns the list of libretro core base names that can handle
// the given game's platform or file extension. Intended for the core-selector UI on the game page.
func (a *App) GetCoresForGame(id uint) ([]string, error) {
	game, err := a.rommSrv.GetRom(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get ROM info: %w", err)
	}

	// Strategy 1 & 2: Platform-based lookup.
	if cores := a.getCoresByPlatform(&game); len(cores) > 0 {
		return cores, nil
	}

	// Strategy 3: Derive extension from the server-side filename (Fallback).
	ext := strings.ToLower(filepath.Ext(filepath.Base(game.FullPath)))
	if ext != ".zip" {
		if cores := retroarch.GetCoresForExt(ext); len(cores) > 0 {
			return cores, nil
		}
	}

	// Strategy 4: Local file scan for the real extension (Fallback for zips).
	if cores := a.getCoresByLocalScan(&game); len(cores) > 0 {
		return cores, nil
	}

	return nil, fmt.Errorf("no known cores for game %d (platform/ext not found)", id)
}

func (a *App) getCoresByPlatform(game *types.Game) []string {
	// Strategy 1: Direct Platform-based lookup (Primary).
	if game.Platform.Slug != "" {
		if cores := retroarch.GetCoresForPlatform(game.Platform.Slug); len(cores) > 0 {
			return cores
		}
	}

	// Strategy 2: Platform-based lookup from path segments.
	fullPath := filepath.ToSlash(game.FullPath)
	parts := strings.Split(strings.TrimPrefix(fullPath, "/"), "/")
	for _, part := range parts {
		if cores := retroarch.GetCoresForPlatform(part); len(cores) > 0 {
			return cores
		}
	}
	return nil
}

func (a *App) getCoresByLocalScan(game *types.Game) []string {
	romDir := a.librarySrv.GetRomDir(game)
	if romDir == "" {
		return nil
	}
	entries, err := os.ReadDir(romDir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		itemPath := filepath.Join(romDir, entry.Name())
		localExt := strings.ToLower(filepath.Ext(entry.Name()))

		if localExt == ".zip" {
			if zipCores := retroarch.GetCoresFromZip(itemPath); len(zipCores) > 0 {
				return zipCores
			}
		} else if localCores := retroarch.GetCoresForExt(localExt); len(localCores) > 0 {
			return localCores
		}
	}
	return nil
}

// --- Internal Provider Implementations ---

func (a *App) ConfigGetConfig() types.AppConfig {
	return a.configManager.GetConfig()
}

func (a *App) ConfigSave(cfg *types.AppConfig) error {
	return a.configManager.Save(cfg)
}

func (a *App) GetRomMHost() string {
	return a.configManager.GetConfig().RommHost
}

func (a *App) GetUsername() string {
	return a.configManager.GetConfig().Username
}

func (a *App) GetPassword() string {
	return a.configManager.GetConfig().Password
}

func (a *App) GetLibraryPath() string {
	return a.configManager.GetConfig().LibraryPath
}

func (a *App) SaveDefaultLibraryPath(path string) error {
	cfg := a.configManager.GetConfig()
	cfg.LibraryPath = path
	return a.configManager.Save(&cfg)
}

func (a *App) GetRetroArchPath() string {
	return a.configManager.GetConfig().RetroArchPath
}

func (a *App) GetCheevosCredentials() (username, password string) {
	cfg := a.configManager.GetConfig()
	return cfg.CheevosUsername, cfg.CheevosPassword
}

func (a *App) GetRom(id uint) (types.Game, error) {
	return a.rommSrv.GetRom(id)
}

func (a *App) DownloadFile(game *types.Game) (reader io.ReadCloser, filename string, err error) {
	return a.rommSrv.GetClient().DownloadFile(game)
}

func (a *App) RomMUploadSave(id uint, core, filename string, content []byte) error {
	return a.rommSrv.GetClient().UploadSave(id, core, filename, content)
}

func (a *App) RomMUploadState(id uint, core, filename string, content []byte) error {
	return a.rommSrv.GetClient().UploadState(id, core, filename, content)
}

func (a *App) RomMDownloadSave(filePath string) (reader io.ReadCloser, filename string, err error) {
	return a.rommSrv.GetClient().DownloadSave(filePath)
}

func (a *App) RomMDownloadState(filePath string) (reader io.ReadCloser, filename string, err error) {
	return a.rommSrv.GetClient().DownloadState(filePath)
}

func (a *App) GetRomDir(game *types.Game) string {
	return a.librarySrv.GetRomDir(game)
}

func (a *App) LogInfof(format string, args ...interface{}) {
	if a.ctx != nil {
		wailsRuntime.LogInfof(a.ctx, format, args...)
	}
}

func (a *App) LogErrorf(format string, args ...interface{}) {
	if a.ctx != nil {
		wailsRuntime.LogErrorf(a.ctx, format, args...)
	}
}

func (a *App) EventsEmit(eventName string, args ...interface{}) {
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, eventName, args...)
	}
}

func (a *App) WindowHide() {
	if a.ctx != nil {
		wailsRuntime.WindowHide(a.ctx)
	}
}

func (a *App) WindowShow() {
	if a.ctx != nil {
		wailsRuntime.WindowShow(a.ctx)
	}
}

func (a *App) WindowUnminimise() {
	if a.ctx != nil {
		wailsRuntime.WindowUnminimise(a.ctx)
	}
}

func (a *App) WindowSetAlwaysOnTop(b bool) {
	if a.ctx != nil {
		wailsRuntime.WindowSetAlwaysOnTop(a.ctx, b)
	}
}

func (a *App) OpenFileDialog(title string, filters []string) (string, error) {
	options := wailsRuntime.OpenDialogOptions{
		Title: title,
	}
	if len(filters) > 0 {
		options.Filters = []wailsRuntime.FileFilter{{DisplayName: "Filtered Files", Pattern: filters[0]}}
	}
	if runtime.GOOS == constants.OSDarwin {
		options.DefaultDirectory = "/Applications"
		options.TreatPackagesAsDirectories = false
		options.Filters = nil
	}
	if a.ctx == nil {
		return "", nil
	}
	return wailsRuntime.OpenFileDialog(a.ctx, options)
}

func (a *App) OpenDirectoryDialog(title string) (string, error) {
	options := wailsRuntime.OpenDialogOptions{
		Title:                title,
		CanCreateDirectories: true,
	}
	if a.ctx == nil {
		return "", nil
	}
	return wailsRuntime.OpenDirectoryDialog(a.ctx, options)
}

// Lifecycle
func (a *App) Quit() {
	wailsRuntime.Quit(a.ctx)
}

func (a *App) Greet(name string) string {
	return "Hello! Go-RomM-Sync is ready."
}
