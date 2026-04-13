package main

import (
	"context"
	"fmt"
	"go-romm-sync/authsrv"
	"go-romm-sync/config"
	"go-romm-sync/configsrv"
	"go-romm-sync/constants"
	"go-romm-sync/launcher"
	"go-romm-sync/library"
	"go-romm-sync/retroarch"
	"go-romm-sync/rommsrv"
	syncSrvPkg "go-romm-sync/sync"
	"go-romm-sync/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx           context.Context
	configManager *config.ConfigManager
	configSrv     *configsrv.Service
	rommSrv       *rommsrv.Service
	librarySrv    *library.Service
	syncSrv       *syncSrvPkg.Service
	launcher      *launcher.Launcher
	authSrv       *authsrv.Service
	coreResolver  *retroarch.CoreResolver

	// Download/Auth protection
	downloadCancels map[uint]context.CancelFunc
	downloadMu      sync.Mutex
	loginMu         sync.Mutex
}

// NewApp creates a new App application struct
func NewApp(cm *config.ConfigManager) *App {
	app := &App{
		configManager:   cm,
		downloadCancels: make(map[uint]context.CancelFunc),
	}
	app.configSrv = configsrv.New(app, app)
	app.rommSrv = rommsrv.New(app)
	app.librarySrv = library.New(app, app, app)
	app.syncSrv = syncSrvPkg.New(app, app, app)
	app.launcher = launcher.New(app, app, app, app)
	app.authSrv = authsrv.New(app.configManager, app.rommSrv, app)
	app.coreResolver = retroarch.NewCoreResolver(func(fullPath string) string {
		// Build a synthetic game to get the rom dir via librarySrv
		g := &types.Game{FullPath: fullPath}
		return app.librarySrv.GetRomDir(g)
	})
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
		updateIfNotEmpty(&current.ClientToken, cfg.ClientToken)

		if current.RommHost != oldHost || current.Username != oldUser || current.Password != oldPass {
			hostOrCredsChanged = true
		}
	})

	if err != nil {
		return fmt.Sprintf("Error saving config: %v", err)
	}

	if hostOrCredsChanged {
		a.rommSrv = rommsrv.New(a)
		a.authSrv = authsrv.New(a.configManager, a.rommSrv, a)
	}

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

// RomM / Auth
func (a *App) Login() (string, error) {
	a.loginMu.Lock()
	defer a.loginMu.Unlock()
	return a.authSrv.Login()
}

func (a *App) Logout() error {
	return a.authSrv.Logout()
}

func (a *App) ClearImageCache() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home dir: %w", err)
	}

	cacheDirs := []string{
		filepath.Join(homeDir, constants.AppDir, constants.CacheDir, constants.CoversDir),
		filepath.Join(homeDir, constants.AppDir, constants.CacheDir, constants.PlatformsDir),
	}

	for _, dir := range cacheDirs {
		if err := os.RemoveAll(dir); err != nil {
			a.LogErrorf("Failed to clear cache directory %s: %v", dir, err)
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			a.LogErrorf("Failed to recreate cache directory %s: %v", dir, err)
		}
	}

	return nil
}

func (a *App) GetLibrary(limit, offset, platformID int, search string) (types.LibraryResult[types.Game], error) {
	for {
		cfg := a.configManager.GetConfig()
		if cfg.OfflineMode {
			items, total, err := a.librarySrv.GetLocalLibrary(limit, offset, platformID, search)
			if err != nil {
				return types.LibraryResult[types.Game]{}, err
			}
			return types.LibraryResult[types.Game]{Items: items, Total: total}, nil
		}

		items, total, err := a.rommSrv.GetLibrary(limit, offset, platformID, search)
		if err != nil {
			a.handleConnectionError(err)
			continue
		}
		return types.LibraryResult[types.Game]{Items: items, Total: total}, nil
	}
}

func (a *App) GetPlatforms(limit, offset int) (types.LibraryResult[types.Platform], error) {
	for {
		cfg := a.configManager.GetConfig()
		if cfg.OfflineMode {
			return a.getOfflinePlatforms(limit, offset)
		}

		items, total, err := a.rommSrv.GetPlatforms(limit, offset)
		if err != nil {
			a.handleConnectionError(err)
			continue
		}
		return types.LibraryResult[types.Platform]{Items: items, Total: total}, nil
	}
}

func (a *App) getOfflinePlatforms(limit, offset int) (types.LibraryResult[types.Platform], error) {
	items, _, err := a.librarySrv.GetLocalLibrary(1000, 0, 0, "")
	if err != nil {
		return types.LibraryResult[types.Platform]{}, err
	}
	platformMap := make(map[uint]types.Platform)
	for i := range items {
		game := &items[i]
		if _, ok := platformMap[game.PlatformID]; ok {
			continue
		}
		platform := game.Platform
		if platform.ID == 0 {
			platform.ID = game.PlatformID
		}
		if platform.Name == "" {
			platform.Name = game.PlatformDisplayName
		}
		if platform.Slug == "" {
			platform.Slug = game.PlatformSlug
		}
		platformMap[game.PlatformID] = platform
	}
	var platforms []types.Platform
	for _, p := range platformMap {
		platforms = append(platforms, p)
	}
	total := len(platforms)
	start := offset
	if start > total {
		start = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return types.LibraryResult[types.Platform]{Items: platforms[start:end], Total: total}, nil
}

func (a *App) GetFirmware(platformID uint) ([]types.Firmware, error) {
	return a.rommSrv.GetFirmware(platformID)
}

func (a *App) SetPlatformFirmware(platformSlug string, firmware *types.Firmware) error {
	cfg := a.configManager.GetConfig()
	if cfg.PlatformFirmware == nil {
		cfg.PlatformFirmware = make(map[string]uint)
	}
	cfg.PlatformFirmware[platformSlug] = firmware.ID
	if err := a.configManager.Save(&cfg); err != nil {
		return err
	}
	if firmware.ID == 0 {
		return a.librarySrv.CleanupFirmware(platformSlug)
	}
	return a.librarySrv.DownloadFirmware(platformSlug, firmware)
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
	parentCtx := a.ctx
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	a.downloadMu.Lock()
	a.downloadCancels[id] = cancel
	a.downloadMu.Unlock()

	defer func() {
		a.downloadMu.Lock()
		delete(a.downloadCancels, id)
		a.downloadMu.Unlock()
	}()

	return a.librarySrv.DownloadRomToLibrary(ctx, id)
}

func (a *App) CancelDownload(id uint) {
	a.downloadMu.Lock()
	cancel, ok := a.downloadCancels[id]
	a.downloadMu.Unlock()

	if ok {
		a.LogInfof("Cancelling download for game ID %d", id)
		cancel()
	}
}

func (a *App) GetRomDownloadStatus(id uint) (bool, error) {
	cfg := a.configManager.GetConfig()
	if cfg.OfflineMode {
		_, err := a.librarySrv.GetLocalGame(id)
		return err == nil, nil
	}
	return a.librarySrv.GetRomDownloadStatus(id)
}

func (a *App) DeleteRom(id uint) error {
	return a.librarySrv.DeleteRom(id)
}

func (a *App) OpenGameFolder(game *types.Game) error {
	romDir := a.librarySrv.GetRomDir(game)
	if _, err := os.Stat(romDir); os.IsNotExist(err) {
		return fmt.Errorf("folder does not exist: %s", romDir)
	}

	absPath, err := filepath.Abs(romDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", absPath)
	case "darwin":
		cmd = exec.Command("open", absPath)
	case "linux":
		cmd = exec.Command("xdg-open", absPath)
	default:
		wailsRuntime.BrowserOpenURL(a.ctx, "file://"+absPath)
		return nil
	}
	a.LogInfof("Opening folder on %s: %s", runtime.GOOS, absPath)
	return cmd.Start()
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

func (a *App) DownloadServerSave(gameID, serverID uint, core, filename, updatedAt string) error {
	return a.syncSrv.DownloadServerSave(gameID, serverID, core, filename, updatedAt)
}

func (a *App) DownloadServerState(gameID, serverID uint, core, filename, updatedAt string) error {
	return a.syncSrv.DownloadServerState(gameID, serverID, core, filename, updatedAt)
}

func (a *App) ValidateAssetPath(core, filename string) (coreBase, fileBase string, err error) {
	return a.syncSrv.ValidateAssetPath(core, filename)
}

// Launch
func (a *App) checkAndDownloadFirmware(id uint) error {
	game, err := a.GetRom(id)
	if err != nil {
		return err
	}

	platformSlug := a.GetResolvedPlatformSlug(&game)
	if platformSlug == "" {
		return nil
	}

	cfg := a.configManager.GetConfig()
	firmwareID, ok := cfg.PlatformFirmware[platformSlug]
	if !ok || firmwareID == 0 {
		return nil
	}

	firmwares, err := a.GetFirmware(game.PlatformID)
	if err != nil {
		a.LogErrorf("Failed to get firmwares from server: %v", err)
		return nil
	}

	var selectedFw *types.Firmware
	for i := range firmwares {
		if firmwares[i].ID == firmwareID {
			selectedFw = &firmwares[i]
			break
		}
	}
	if selectedFw == nil {
		a.LogErrorf("Selected firmware ID %d not found in available firmwares", firmwareID)
		return nil
	}

	if !a.librarySrv.IsFirmwareDownloaded(platformSlug, selectedFw) {
		a.LogInfof("Firmware %s is missing locally. Attempting to download...", selectedFw.FileName)
		a.EventsEmit(constants.EventPlayStatus, "Downloading missing firmware for platform...")
		if err := a.librarySrv.DownloadFirmware(platformSlug, selectedFw); err != nil {
			a.LogErrorf("Failed to auto-download firmware: %v", err)
			return err
		}
		a.LogInfof("Successfully auto-downloaded firmware %s", selectedFw.FileName)
	}

	return nil
}

func (a *App) PlayRom(id uint) error {
	if err := a.checkAndDownloadFirmware(id); err != nil {
		a.LogErrorf("Firmware check failed: %v", err)
	}
	return a.launcher.PlayRom(id)
}

func (a *App) PlayRomWithCore(id uint, coreName string) error {
	if err := a.checkAndDownloadFirmware(id); err != nil {
		a.LogErrorf("Firmware check failed: %v", err)
	}
	return a.launcher.PlayRomWithCore(id, coreName)
}

func (a *App) ToggleOfflineMode() bool {
	var newState bool
	if err := a.configManager.Update(func(cfg *types.AppConfig) {
		cfg.OfflineMode = !cfg.OfflineMode
		newState = cfg.OfflineMode
	}); err != nil {
		a.LogErrorf("Failed to update config during ToggleOfflineMode: %v", err)
	}
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "offline-mode-changed", newState)
	}
	return newState
}

func (a *App) handleConnectionError(err error) {
	if err == nil {
		return
	}
	a.LogErrorf("Server operation failed: %v. Automatically switching to offline mode.", err)
	if err := a.configManager.Update(func(cfg *types.AppConfig) {
		cfg.OfflineMode = true
	}); err != nil {
		a.LogErrorf("Failed to update config during handleConnectionError: %v", err)
	}
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "offline-mode-changed", true)
	}
}

func (a *App) UpdateRetroArchCores() error {
	cfg := a.configManager.GetConfig()
	if cfg.RetroArchPath == "" {
		return fmt.Errorf("retroarch executable not configured")
	}
	return retroarch.UpdateAllCores(a, cfg.RetroArchPath)
}

func (a *App) UpdateRetroArchBios() error {
	cfg := a.configManager.GetConfig()
	if cfg.RetroArchPath == "" {
		return fmt.Errorf("retroarch executable not configured")
	}
	return retroarch.UpdateBios(a, cfg.RetroArchPath)
}

func (a *App) SyncOfflineMetadata() error {
	const batchSize = 100
	offset := 0
	for {
		batch, total, err := a.rommSrv.GetLibrary(batchSize, offset, 0, "")
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}
		for i := range batch {
			game := &batch[i]
			status, err := a.librarySrv.GetRomDownloadStatus(game.ID)
			if err != nil {
				a.LogErrorf("Failed to get download status for game %d: %v", game.ID, err)
				continue
			}
			if status {
				if err := a.librarySrv.SaveMetadata(game); err != nil {
					a.LogErrorf("Failed to save metadata for game %d: %v", game.ID, err)
				}
			}
		}
		offset += batchSize
		if offset >= total {
			break
		}
	}
	return nil
}

// GetCoresForGame returns an ordered list of candidate cores for a game,
// delegating to CoreResolver for the multi-strategy fallback chain.
func (a *App) GetCoresForGame(id uint) ([]string, error) {
	cfg := a.configManager.GetConfig()

	var game types.Game
	var err error
	if cfg.OfflineMode {
		game, err = a.librarySrv.GetLocalGame(id)
	} else {
		game, err = a.rommSrv.GetRom(id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get ROM info: %w", err)
	}

	platformSlug := a.GetResolvedPlatformSlug(&game)
	lastUsed := ""
	if platformSlug != "" {
		lastUsed = cfg.LastUsedCores[platformSlug]
	}

	cores := a.coreResolver.Resolve(retroarch.ResolveOptions{
		PlatformSlug: platformSlug,
		FullPath:     game.FullPath,
		LastUsed:     lastUsed,
	})
	if len(cores) == 0 {
		return nil, fmt.Errorf("no known cores for game %d (platform/ext not found)", id)
	}
	return cores, nil
}

// GetResolvedPlatformSlug returns a canonical platform slug, falling back to folder name if needed.
func (a *App) GetResolvedPlatformSlug(game *types.Game) string {
	if game.Platform.Slug != "" {
		return game.Platform.Slug
	}
	relDir := filepath.Dir(game.FullPath)
	parts := strings.Split(filepath.ToSlash(relDir), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if slug := retroarch.IdentifyPlatform(parts[i]); slug != "" {
			return slug
		}
	}
	return ""
}

// SaveLastUsedCore saves the core choice for a platform.
func (a *App) SaveLastUsedCore(platformSlug, coreName string) error {
	if platformSlug == "" || coreName == "" {
		return nil
	}
	return a.configManager.Update(func(cfg *types.AppConfig) {
		if cfg.LastUsedCores == nil {
			cfg.LastUsedCores = make(map[string]string)
		}
		cfg.LastUsedCores[platformSlug] = coreName
	})
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

func (a *App) GetClientToken() string {
	return a.configManager.GetConfig().ClientToken
}

func (a *App) GetLibraryPath() string {
	return a.configManager.GetConfig().LibraryPath
}

func (a *App) GetBiosDir() string {
	return a.librarySrv.GetBiosDir()
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
	for {
		cfg := a.configManager.GetConfig()
		if cfg.OfflineMode {
			return a.librarySrv.GetLocalGame(id)
		}
		game, err := a.rommSrv.GetRom(id)
		if err != nil {
			a.handleConnectionError(err)
			continue
		}
		return game, nil
	}
}

func (a *App) DownloadFile(ctx context.Context, game *types.Game) (reader io.ReadCloser, filename string, err error) {
	return a.rommSrv.GetClient().DownloadFile(ctx, game)
}

func (a *App) DownloadFirmwareContent(ctx context.Context, id uint, fileName string) (io.ReadCloser, string, error) {
	return a.rommSrv.GetClient().DownloadFirmwareContent(ctx, id, fileName)
}

func (a *App) GetLocalGame(id uint) (types.Game, error) {
	return a.librarySrv.GetLocalGame(id)
}

func (a *App) RomMUploadSave(id uint, core, filename string, content []byte) error {
	return a.rommSrv.GetClient().UploadSave(id, core, filename, content)
}

func (a *App) RomMUploadState(id uint, core, filename string, content []byte) error {
	return a.rommSrv.GetClient().UploadState(id, core, filename, content)
}

func (a *App) RomMDownloadSave(ctx context.Context, id uint) (reader io.ReadCloser, filename string, err error) {
	return a.rommSrv.GetClient().DownloadSave(ctx, id)
}

func (a *App) RomMDownloadState(ctx context.Context, id uint) (reader io.ReadCloser, filename string, err error) {
	return a.rommSrv.GetClient().DownloadState(ctx, id)
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
	options := wailsRuntime.OpenDialogOptions{Title: title}
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
