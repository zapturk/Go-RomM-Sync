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
	"runtime"
	"strings"

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
	app.syncSrv = sync.New(app, app)
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

func (a *App) SaveConfig(cfg types.AppConfig) string {
	res, hostChanged := a.configSrv.SaveConfig(cfg)

	// Clear RetroArch cheevos token on save to ensure fresh login on credentials change
	fullCfg := a.configManager.GetConfig()
	if err := retroarch.ClearCheevosToken(fullCfg.RetroArchPath); err != nil {
		a.LogErrorf("Failed to clear RetroArch cheevos token: %v", err)
	}

	if hostChanged {
		a.rommSrv = rommsrv.New(a)
	}
	return res
}

func (a *App) SelectRetroArchExecutable() (string, error) {
	return a.configSrv.SelectRetroArchExecutable()
}

func (a *App) SelectLibraryPath() (string, error) {
	return a.configSrv.SelectLibraryPath()
}

// RomM
func (a *App) Login() (string, error) {
	return a.rommSrv.Login()
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
	return a.syncSrv.DeleteGameFile(id, "saves", core, filename)
}

func (a *App) DeleteState(id uint, core, filename string) error {
	return a.syncSrv.DeleteGameFile(id, "states", core, filename)
}

func (a *App) UploadSave(id uint, core, filename string) error {
	return a.syncSrv.UploadSave(id, core, filename)
}

func (a *App) UploadState(id uint, core, filename string) error {
	return a.syncSrv.UploadState(id, core, filename)
}

func (a *App) DownloadServerSave(gameID uint, filePath string, core string, filename string) error {
	return a.syncSrv.DownloadServerSave(gameID, filePath, core, filename)
}

func (a *App) DownloadServerState(gameID uint, filePath string, core string, filename string) error {
	return a.syncSrv.DownloadServerState(gameID, filePath, core, filename)
}

func (a *App) ValidateAssetPath(core, filename string) (string, string, error) {
	return a.syncSrv.ValidateAssetPath(core, filename)
}

// Launch
func (a *App) PlayRom(id uint) error {
	return a.launcher.PlayRom(id)
}

// --- Internal Provider Implementations ---

func (a *App) ConfigGetConfig() types.AppConfig {
	return a.configManager.GetConfig()
}

func (a *App) ConfigSave(cfg types.AppConfig) error {
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
	return a.configManager.Save(cfg)
}

func (a *App) GetRetroArchPath() string {
	return a.configManager.GetConfig().RetroArchPath
}

func (a *App) GetCheevosCredentials() (string, string) {
	cfg := a.configManager.GetConfig()
	return cfg.CheevosUsername, cfg.CheevosPassword
}

func (a *App) GetRom(id uint) (types.Game, error) {
	return a.rommSrv.GetRom(id)
}

func (a *App) DownloadFile(game *types.Game) (io.ReadCloser, string, error) {
	return a.rommSrv.GetClient().DownloadFile(game)
}

func (a *App) RomMUploadSave(id uint, core, filename string, content []byte) error {
	return a.rommSrv.GetClient().UploadSave(id, core, filename, content)
}

func (a *App) RomMUploadState(id uint, core, filename string, content []byte) error {
	return a.rommSrv.GetClient().UploadState(id, core, filename, content)
}

func (a *App) RomMDownloadSave(filePath string) (io.ReadCloser, string, error) {
	return a.rommSrv.GetClient().DownloadSave(filePath)
}

func (a *App) RomMDownloadState(filePath string) (io.ReadCloser, string, error) {
	return a.rommSrv.GetClient().DownloadState(filePath)
}

func (a *App) GetRomDir(game *types.Game) string {
	return a.librarySrv.GetRomDir(game)
}

func (a *App) LogInfof(format string, args ...interface{}) {
	wailsRuntime.LogInfof(a.ctx, format, args...)
}

func (a *App) LogErrorf(format string, args ...interface{}) {
	wailsRuntime.LogErrorf(a.ctx, format, args...)
}

func (a *App) EventsEmit(eventName string, args ...interface{}) {
	wailsRuntime.EventsEmit(a.ctx, eventName, args...)
}

func (a *App) WindowHide() {
	wailsRuntime.WindowHide(a.ctx)
}

func (a *App) WindowShow() {
	wailsRuntime.WindowShow(a.ctx)
}

func (a *App) WindowUnminimise() {
	wailsRuntime.WindowUnminimise(a.ctx)
}

func (a *App) WindowSetAlwaysOnTop(b bool) {
	wailsRuntime.WindowSetAlwaysOnTop(a.ctx, b)
}

func (a *App) OpenFileDialog(title string, filters []string) (string, error) {
	options := wailsRuntime.OpenDialogOptions{
		Title: title,
	}
	if len(filters) > 0 {
		options.Filters = []wailsRuntime.FileFilter{{DisplayName: "Filtered Files", Pattern: filters[0]}}
	}
	if runtime.GOOS == "darwin" {
		options.DefaultDirectory = "/Applications"
		options.TreatPackagesAsDirectories = false
		options.Filters = nil
	}
	return wailsRuntime.OpenFileDialog(a.ctx, options)
}

func (a *App) OpenDirectoryDialog(title string) (string, error) {
	options := wailsRuntime.OpenDialogOptions{
		Title:                title,
		CanCreateDirectories: true,
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
