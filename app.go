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
	syncSrvPkg "go-romm-sync/sync"
	"go-romm-sync/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync" // Added for download cancellation map protection

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
	syncSrv       *syncSrvPkg.Service
	launcher      *launcher.Launcher

	// Download cancellation support
	downloadCancels map[uint]context.CancelFunc
	downloadMu      sync.Mutex
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
		// Re-create it so future operations don't fail if they expect it
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
			return types.LibraryResult[types.Game]{
				Items: items,
				Total: total,
			}, nil
		}

		items, total, err := a.rommSrv.GetLibrary(limit, offset, platformID, search)
		if err != nil {
			a.handleConnectionError(err)
			continue
		}
		return types.LibraryResult[types.Game]{
			Items: items,
			Total: total,
		}, nil
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
		return types.LibraryResult[types.Platform]{
			Items: items,
			Total: total,
		}, nil
	}
}

func (a *App) getOfflinePlatforms(limit, offset int) (types.LibraryResult[types.Platform], error) {
	// For now, simplicity: scan local library for platforms
	// Alternatively, we could save platform metadata too, but scanning works for now.
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
		// Fill in missing fields from the game struct
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
	// Basic paging for offline platforms
	total := len(platforms)
	start := offset
	if start > total {
		start = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return types.LibraryResult[types.Platform]{
		Items: platforms[start:end],
		Total: total,
	}, nil
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
	err := a.configManager.Save(&cfg)
	if err != nil {
		return err
	}

	// If ID is 0, we are unsetting the firmware.
	if firmware.ID == 0 {
		return a.librarySrv.CleanupFirmware(platformSlug)
	}

	// Trigger download
	return a.librarySrv.DownloadFirmware(firmware)
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
	// 1. Create a cancellable context
	parentCtx := a.ctx
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	// 2. Store the cancel function
	a.downloadMu.Lock()
	a.downloadCancels[id] = cancel
	a.downloadMu.Unlock()

	// 3. Ensure cleanup when done
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
		a.LogInfof("Opening folder on Windows: %s", absPath)
		cmd = exec.Command("explorer", absPath)
	case "darwin":
		a.LogInfof("Opening folder on macOS: %s", absPath)
		cmd = exec.Command("open", absPath)
	case "linux":
		a.LogInfof("Opening folder on Linux: %s", absPath)
		cmd = exec.Command("xdg-open", absPath)
	default:
		// Fallback to cross-platform Wails helper for other OSs
		a.LogInfof("Opening folder via BrowserOpenURL: %s", absPath)
		fileURL := "file://" + absPath
		wailsRuntime.BrowserOpenURL(a.ctx, fileURL)
		return nil
	}

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

// DownloadServerSave downloads a remote save file into the local library structure
func (a *App) DownloadServerSave(gameID, serverID uint, core, filename, updatedAt string) error {
	return a.syncSrv.DownloadServerSave(gameID, serverID, core, filename, updatedAt)
}

// DownloadServerState downloads a remote state file into the local library structure
func (a *App) DownloadServerState(gameID, serverID uint, core, filename, updatedAt string) error {
	return a.syncSrv.DownloadServerState(gameID, serverID, core, filename, updatedAt)
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

func (a *App) SyncOfflineMetadata() error {
	// Fetches metadata from server for all currently downloaded games
	// and saves it locally. This helps migrate existing libraries.
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

func (a *App) GetCoresForGame(id uint) ([]string, error) {
	var game types.Game
	var err error

	cfg := a.configManager.GetConfig()
	if cfg.OfflineMode {
		game, err = a.librarySrv.GetLocalGame(id)
	} else {
		game, err = a.rommSrv.GetRom(id)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get ROM info: %w", err)
	}

	// Resolve the platform slug robustly
	platformSlug := a.GetResolvedPlatformSlug(&game)

	// Strategy 0: User Preference (Last Used Core for this platform).
	lastUsed := ""
	if platformSlug != "" {
		lastUsed = cfg.LastUsedCores[platformSlug]
	}

	var allCores []string

	// Strategy 1 & 2: Platform-based lookup.
	if cores := a.getCoresByPlatform(&game); len(cores) > 0 {
		allCores = append(allCores, cores...)
	}

	// Strategy 3: Derive extension from the server-side filename (Fallback).
	if len(allCores) == 0 {
		ext := strings.ToLower(filepath.Ext(filepath.Base(game.FullPath)))
		if ext != ".zip" {
			if cores := retroarch.GetCoresForExt(ext); len(cores) > 0 {
				allCores = append(allCores, cores...)
			}
		}
	}

	// Strategy 4: Local file scan for the real extension (Fallback for zips).
	if len(allCores) == 0 {
		if cores := a.getCoresByLocalScan(&game); len(cores) > 0 {
			allCores = append(allCores, cores...)
		}
	}

	if len(allCores) == 0 {
		return nil, fmt.Errorf("no known cores for game %d (platform/ext not found)", id)
	}

	return a.prioritizeLastUsedCore(allCores, lastUsed), nil
}

func (a *App) prioritizeLastUsedCore(allCores []string, lastUsed string) []string {
	if lastUsed == "" {
		return allCores
	}
	finalCores := []string{lastUsed}
	for _, c := range allCores {
		if c != lastUsed {
			finalCores = append(finalCores, c)
		}
	}
	return finalCores
}

// GetResolvedPlatformSlug returns a canonical platform slug, falling back to folder name if needed.
func (a *App) GetResolvedPlatformSlug(game *types.Game) string {
	if game.Platform.Slug != "" {
		return game.Platform.Slug
	}
	// Fallback to directory name identification
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
