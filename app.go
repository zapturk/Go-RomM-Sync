package main

import (
	"context"
	"fmt"
	"go-romm-sync/assets"
	"go-romm-sync/authsrv"
	"go-romm-sync/config"
	"go-romm-sync/constants"
	"go-romm-sync/firmware"
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
	"sort"
	"strconv"
	"strings"
	"sync"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx           context.Context
	configManager *config.ConfigManager
	rommSrv       *rommsrv.Service
	librarySrv    *library.Service
	syncSrv       *syncSrvPkg.Service
	authSrv       *authsrv.Service
	coreResolver  *retroarch.CoreResolver
	firmwareSrv   *firmware.Service
	assetSrv      *assets.Service

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
	app.rommSrv = rommsrv.New(app)
	app.librarySrv = library.New(app.configManager, app.rommSrv, app)
	app.syncSrv = syncSrvPkg.New(app.librarySrv, app.rommSrv, app)
	app.authSrv = authsrv.New(app.configManager, app.rommSrv, app)
	app.firmwareSrv = firmware.New(app.configManager, app.rommSrv, app)
	app.assetSrv = assets.New(app, app.rommSrv, app)
	app.coreResolver = retroarch.NewCoreResolver(app.librarySrv)
	return app
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// --- Wails External API (Thin Wrappers) ---

// Config
func (a *App) GetConfig() types.AppConfig {
	return a.configManager.GetConfig()
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
		updateIfNotEmpty(&current.ThemeBackground, cfg.ThemeBackground)
		updateIfNotEmpty(&current.ThemeFont, cfg.ThemeFont)
		updateIfNotEmpty(&current.ThemeTextColor, cfg.ThemeTextColor)
		updateIfNotEmpty(&current.ThemeBtnTextColor, cfg.ThemeBtnTextColor)
		if cfg.ThemeBackground != "" {
			current.ThemeCustomBackground = cfg.ThemeCustomBackground
		}

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
	filters := []string{"*.*"}
	if runtime.GOOS != "darwin" {
		filters = append(filters, "*.exe;*.app;retroarch")
	}

	selectedFile, err := a.OpenFileDialog("Select RetroArch Executable", filters)
	if err != nil {
		return "", err
	}

	if selectedFile != "" {
		cfg := a.configManager.GetConfig()
		cfg.RetroArchPath = selectedFile
		if err = a.configManager.Save(&cfg); err != nil {
			return "", fmt.Errorf("failed to save config: %w", err)
		}
	}

	return selectedFile, nil
}

// DownloadAndInstallRetroArch downloads, installs, and configures the RetroArch executable
// for the current operating system and CPU architecture.
func (a *App) DownloadAndInstallRetroArch() (string, error) {
	installedPath, err := retroarch.DownloadAndInstall(a)
	if err != nil {
		a.LogErrorf("Failed to download and install RetroArch: %v", err)
		return "", err
	}

	cfg := a.configManager.GetConfig()
	cfg.RetroArchPath = installedPath
	if err = a.configManager.Save(&cfg); err != nil {
		return installedPath, fmt.Errorf("retroarch installed at %s but failed to save config: %w", installedPath, err)
	}

	if err := retroarch.ClearCheevosToken(installedPath); err != nil {
		a.LogErrorf("Failed to clear RetroArch cheevos token: %v", err)
	}

	a.EventsEmit("config-updated", nil)
	return installedPath, nil
}

func (a *App) SelectLibraryPath() (string, error) {
	selectedDir, err := a.OpenDirectoryDialog("Select ROM Library Directory")
	if err != nil {
		return "", err
	}

	if selectedDir != "" {
		cfg := a.configManager.GetConfig()
		cfg.LibraryPath = selectedDir
		if err = a.configManager.Save(&cfg); err != nil {
			return "", fmt.Errorf("failed to save config: %w", err)
		}
	}

	return selectedDir, nil
}

func (a *App) GetDefaultLibraryPath() (string, error) {
	return config.GetDefaultLibraryPath()
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
	return a.assetSrv.ClearCache()
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

// GetPlatform fetches a single platform by its ID.
func (a *App) GetPlatform(platformID uint) (types.Platform, error) {
	for {
		cfg := a.configManager.GetConfig()
		if cfg.OfflineMode {
			items, _, err := a.librarySrv.GetLocalLibrary(1000, 0, int(platformID), "")
			if err != nil {
				return types.Platform{}, err
			}
			for i := range items {
				game := &items[i]
				if game.PlatformID == platformID {
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
					return platform, nil
				}
			}
			return types.Platform{ID: platformID}, nil
		}

		platform, err := a.rommSrv.GetPlatform(platformID)
		if err != nil {
			a.handleConnectionError(err)
			continue
		}
		return platform, nil
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
	platforms := make([]types.Platform, 0, len(platformMap))
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

func (a *App) SetPlatformFirmware(platformSlug string, fw *types.Firmware) error {
	cfg := a.configManager.GetConfig()
	if cfg.PlatformFirmware == nil {
		cfg.PlatformFirmware = make(map[string]uint)
	}
	cfg.PlatformFirmware[platformSlug] = fw.ID
	if err := a.configManager.Save(&cfg); err != nil {
		return err
	}
	if fw.ID == 0 {
		return a.firmwareSrv.CleanupFirmware(platformSlug)
	}
	return a.firmwareSrv.DownloadFirmware(platformSlug, fw)
}

func (a *App) GetCover(romID uint, coverURL string) (string, error) {
	return a.assetSrv.GetCover(romID, coverURL)
}

func (a *App) GetPlatformCover(platformID uint, slug string) (string, error) {
	return a.assetSrv.GetPlatformCover(platformID, slug)
}

func (a *App) GetServerSaves(id uint) ([]types.ServerSave, error) {
	return a.rommSrv.GetServerSaves(id)
}

func (a *App) GetServerSavesForSlot(id uint, slot string) ([]types.ServerSave, error) {
	return a.rommSrv.GetServerSavesForSlot(id, slot)
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
	slot := a.GetGameSaveSlot(id)
	return a.syncSrv.UploadSave(id, core, filename, slot)
}

func (a *App) UploadSaveToSlot(id uint, core, filename, slot string) error {
	return a.syncSrv.UploadSave(id, core, filename, slot)
}

func (a *App) DeleteServerSave(id uint) error {
	return a.rommSrv.DeleteServerSaves([]uint{id})
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

func (a *App) BridgeGameSaves(id uint, targetCore string) error {
	return a.syncSrv.BridgeGameSaves(id, targetCore)
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

	if !a.firmwareSrv.IsFirmwareDownloaded(platformSlug, selectedFw) {
		a.LogInfof("Firmware %s is missing locally. Attempting to download...", selectedFw.FileName)
		a.EventsEmit(constants.EventPlayStatus, "Downloading missing firmware for platform...")
		if err := a.firmwareSrv.DownloadFirmware(platformSlug, selectedFw); err != nil {
			a.LogErrorf("Failed to auto-download firmware: %v", err)
			return err
		}
		a.LogInfof("Successfully auto-downloaded firmware %s", selectedFw.FileName)
	}

	return nil
}

func (a *App) PlayRomWithCore(id uint, coreOverride string) error {
	if err := a.checkAndDownloadFirmware(id); err != nil {
		a.LogErrorf("Firmware check failed: %v", err)
	}
	if a.GetLibraryPath() == "" {
		return fmt.Errorf("library path is not configured")
	}

	game, err := a.GetRom(id)
	if err != nil {
		return fmt.Errorf("failed to get ROM info: %w", err)
	}

	romDir := a.librarySrv.GetRomDir(&game)
	romPath := a.findRomPath(&game, romDir)
	if romPath == "" {
		return fmt.Errorf("no valid ROM file found in %s, please download it first", romDir)
	}

	exePath, err := a.resolveRetroArchExecutable()
	if err != nil {
		return err
	}

	platformSlug, coreToSave, controllerType := a.resolveCoreAndController(id, &game, coreOverride)
	if coreToSave != "" {
		_ = a.syncSrv.BridgeGameSaves(id, coreToSave)
	}
	cheevosUser, cheevosPass := a.GetCheevosCredentials()

	err = retroarch.Launch(a, exePath, romPath, cheevosUser, cheevosPass, coreOverride, platformSlug, a.GetBiosDir(), controllerType)
	if err != nil {
		return fmt.Errorf("failed to launch game: %w", err)
	}

	return nil
}

func (a *App) resolveRetroArchExecutable() (string, error) {
	exePath := a.GetRetroArchPath()
	if exePath == "" {
		var err error
		exePath, err = a.SelectRetroArchExecutable()
		if err != nil {
			return "", fmt.Errorf("retroarch not configured: %w", err)
		}
		if exePath == "" {
			return "", fmt.Errorf("launch cancelled: RetroArch executable not selected")
		}
		return exePath, nil
	}
	if _, err := os.Stat(exePath); err != nil {
		return "", fmt.Errorf("retroarch executable not found at configured path: %s", exePath)
	}
	return exePath, nil
}

func (a *App) resolveCoreAndController(id uint, game *types.Game, coreOverride string) (platformSlug, coreToSave, controllerType string) {
	platformSlug = a.GetResolvedPlatformSlug(game)
	coreToSave = coreOverride
	if coreToSave == "" {
		if cores := retroarch.GetCoresForPlatform(platformSlug); len(cores) > 0 {
			coreToSave = cores[0]
		}
	}
	if coreToSave != "" && platformSlug != "" {
		_ = a.SaveLastUsedCore(platformSlug, coreToSave)
	}

	if platformSlug == "wii" || strings.Contains(strings.ToLower(platformSlug), "wii") || coreToSave == "dolphin_libretro" {
		controllerType = a.GetGameController(id)
	}
	return platformSlug, coreToSave, controllerType
}

// findRomPath looks for a valid ROM file in the given directory.
func (a *App) findRomPath(game *types.Game, romDir string) string {
	if startupFile := a.GetGameStartupFile(game.ID); startupFile != "" {
		targetPath := filepath.Join(romDir, filepath.Base(startupFile))
		if info, err := os.Stat(targetPath); err == nil && !info.IsDir() {
			return targetPath
		}
	}

	files, err := os.ReadDir(romDir)
	if err != nil {
		return ""
	}

	if a.configManager.GetConfig().UsePlatformFolder {
		files = library.FilterPlatformFolderFiles(files, game)
	}

	if p := a.findCueFile(romDir, files); p != "" {
		return p
	}

	if p := a.findExactMatch(game, romDir); p != "" {
		return p
	}

	if p := a.findPlatformPreferredRom(game, romDir, files); p != "" {
		return p
	}

	return a.findAnyRom(romDir, files)
}

func (a *App) findCueFile(romDir string, files []os.DirEntry) string {
	for _, file := range files {
		if !file.IsDir() && strings.ToLower(filepath.Ext(file.Name())) == constants.ExtCue {
			return filepath.Join(romDir, file.Name())
		}
	}
	return ""
}

func (a *App) findExactMatch(game *types.Game, romDir string) string {
	baseName := filepath.Base(game.FullPath)
	directPath := filepath.Join(romDir, baseName)
	if info, err := os.Stat(directPath); err == nil && !info.IsDir() {
		return directPath
	}
	return ""
}

func (a *App) findPlatformPreferredRom(game *types.Game, romDir string, files []os.DirEntry) string {
	platformSlug := a.GetResolvedPlatformSlug(game)
	platformCores := retroarch.GetCoresForPlatform(platformSlug)
	for _, file := range files {
		if file.IsDir() || strings.HasPrefix(file.Name(), ".") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(file.Name()))

		coreName, ok := retroarch.CoreMap[ext]
		if !ok {
			continue
		}

		for _, pc := range platformCores {
			if pc == coreName {
				return filepath.Join(romDir, file.Name())
			}
		}
	}
	return ""
}

func (a *App) findAnyRom(romDir string, files []os.DirEntry) string {
	for _, file := range files {
		if file.IsDir() || strings.HasPrefix(file.Name(), ".") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(file.Name()))
		if _, ok := retroarch.CoreMap[ext]; ok || ext == ".zip" {
			return filepath.Join(romDir, file.Name())
		}
	}
	return ""
}

func (a *App) ToggleOfflineMode() bool {
	var newState bool
	currentConfig := a.configManager.GetConfig()
	if currentConfig.DisableMetadata {
		a.LogErrorf("ToggleOfflineMode: Blocked offline mode toggle because DisableMetadata is true")
		return false
	}
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

func (a *App) ToggleDisableMetadata() (bool, error) {
	currentConfig := a.configManager.GetConfig()
	targetState := !currentConfig.DisableMetadata

	err := a.configManager.Update(func(cfg *types.AppConfig) {
		cfg.DisableMetadata = targetState
		if targetState {
			cfg.OfflineMode = false
		}
	})
	if err != nil {
		a.LogErrorf("Failed to update config during ToggleDisableMetadata: %v", err)
		return currentConfig.DisableMetadata, err
	}

	if targetState && a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "offline-mode-changed", false)
	}

	a.LogInfof("ToggleDisableMetadata: Successfully changed DisableMetadata to %t", targetState)
	return targetState, nil
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
		GameID:       game.ID,
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
	slugCandidate := game.Platform.Slug
	if slugCandidate == "" {
		slugCandidate = game.PlatformSlug
	}
	if slugCandidate != "" {
		if canonical := retroarch.IdentifyPlatform(slugCandidate); canonical != "" {
			return canonical
		}
		return slugCandidate
	}
	relDir := filepath.Dir(game.FullPath)
	parts := strings.Split(filepath.ToSlash(relDir), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if slug := retroarch.IdentifyPlatform(parts[i]); slug != "" {
			return slug
		}
	}
	if game.PlatformDisplayName != "" {
		if slug := retroarch.IdentifyPlatform(game.PlatformDisplayName); slug != "" {
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

// GetGameController returns the configured controller type ID for the given game,
// or the default Wii controller type if not set.
func (a *App) GetGameController(id uint) string {
	cfg := a.configManager.GetConfig()
	key := strconv.FormatUint(uint64(id), 10)
	if cfg.GameControllers != nil {
		if val, ok := cfg.GameControllers[key]; ok && val != "" {
			return val
		}
	}
	return "769"
}

// SetGameController saves the selected controller type ID for the given game.
func (a *App) SetGameController(id uint, controllerType string) error {
	key := strconv.FormatUint(uint64(id), 10)
	return a.configManager.Update(func(cfg *types.AppConfig) {
		if cfg.GameControllers == nil {
			cfg.GameControllers = make(map[string]string)
		}
		cfg.GameControllers[key] = controllerType
	})
}

// GetGameStartupFile returns the configured startup file name for the given game,
// or empty string if not configured.
func (a *App) GetGameStartupFile(id uint) string {
	cfg := a.configManager.GetConfig()
	key := strconv.FormatUint(uint64(id), 10)
	if cfg.GameStartupFiles != nil {
		if val, ok := cfg.GameStartupFiles[key]; ok && val != "" {
			return val
		}
	}
	return ""
}

// SetGameStartupFile saves the selected startup file name for the given game.
func (a *App) SetGameStartupFile(id uint, fileName string) error {
	key := strconv.FormatUint(uint64(id), 10)
	return a.configManager.Update(func(cfg *types.AppConfig) {
		if cfg.GameStartupFiles == nil {
			cfg.GameStartupFiles = make(map[string]string)
		}
		cfg.GameStartupFiles[key] = fileName
	})
}

// GetGameSaveSlot returns the active save slot for the given game,
// defaulting to "default" if not set.
func (a *App) GetGameSaveSlot(id uint) string {
	cfg := a.configManager.GetConfig()
	key := strconv.FormatUint(uint64(id), 10)
	if cfg.GameSaveSlots != nil {
		if val, ok := cfg.GameSaveSlots[key]; ok && val != "" {
			return val
		}
	}
	return "default"
}

// SetGameSaveSlot saves the selected save slot name for the given game.
func (a *App) SetGameSaveSlot(id uint, slot string) error {
	key := strconv.FormatUint(uint64(id), 10)
	slot = strings.TrimSpace(slot)
	if slot == "" {
		slot = "default"
	}
	return a.configManager.Update(func(cfg *types.AppConfig) {
		if cfg.GameSaveSlots == nil {
			cfg.GameSaveSlots = make(map[string]string)
		}
		cfg.GameSaveSlots[key] = slot
	})
}

// GetSaveSlots returns all available save slots for a game, merging
// server slots, custom local slots, and the active slot.
func (a *App) GetSaveSlots(id uint) ([]types.SaveSlot, error) {
	activeSlot := a.GetGameSaveSlot(id)
	cfg := a.configManager.GetConfig()
	key := strconv.FormatUint(uint64(id), 10)

	slotMap := make(map[string]*types.SaveSlot)
	var slotOrder []string

	if !cfg.OfflineMode && a.rommSrv != nil {
		serverSlots, err := a.rommSrv.GetSaveSlots(id)
		if err == nil {
			for i := range serverSlots {
				s := serverSlots[i]
				slotMap[s.Slot] = &s
				slotOrder = append(slotOrder, s.Slot)
			}
		}
	}

	// Merge locally created custom slots for this game
	if cfg.CustomSaveSlots != nil {
		if customList, ok := cfg.CustomSaveSlots[key]; ok {
			for _, cs := range customList {
				cs = strings.TrimSpace(cs)
				if cs == "" {
					continue
				}
				if _, exists := slotMap[cs]; !exists {
					slotMap[cs] = &types.SaveSlot{
						Slot:            cs,
						Count:           0,
						LatestUpdatedAt: "",
					}
					slotOrder = append(slotOrder, cs)
				}
			}
		}
	}

	// Ensure the active slot is present in the list
	if _, exists := slotMap[activeSlot]; !exists {
		slotMap[activeSlot] = &types.SaveSlot{
			Slot:            activeSlot,
			Count:           0,
			LatestUpdatedAt: "",
		}
		slotOrder = append(slotOrder, activeSlot)
	}

	// Ensure "default" is in the list
	if _, exists := slotMap["default"]; !exists {
		slotMap["default"] = &types.SaveSlot{
			Slot:            "default",
			Count:           0,
			LatestUpdatedAt: "",
		}
		slotOrder = append(slotOrder, "default")
	}

	var result []types.SaveSlot
	for _, name := range slotOrder {
		if s, ok := slotMap[name]; ok {
			s.IsActive = (s.Slot == activeSlot)
			result = append(result, *s)
			delete(slotMap, name)
		}
	}

	return result, nil
}

// CreateSaveSlot creates a new custom save slot for a game and sets it as active.
func (a *App) CreateSaveSlot(id uint, slot string) error {
	slot = strings.TrimSpace(slot)
	if slot == "" {
		return fmt.Errorf("slot name cannot be empty")
	}
	key := strconv.FormatUint(uint64(id), 10)
	return a.configManager.Update(func(cfg *types.AppConfig) {
		if cfg.CustomSaveSlots == nil {
			cfg.CustomSaveSlots = make(map[string][]string)
		}
		exists := false
		for _, existing := range cfg.CustomSaveSlots[key] {
			if strings.EqualFold(existing, slot) {
				exists = true
				break
			}
		}
		if !exists {
			cfg.CustomSaveSlots[key] = append(cfg.CustomSaveSlots[key], slot)
		}
		if cfg.GameSaveSlots == nil {
			cfg.GameSaveSlots = make(map[string]string)
		}
		cfg.GameSaveSlots[key] = slot
	})
}

func (a *App) deleteServerSavesForSlot(id uint, slot string) error {
	cfg := a.configManager.GetConfig()
	if cfg.OfflineMode || a.rommSrv == nil {
		return nil
	}
	serverSaves, err := a.rommSrv.GetServerSaves(id)
	if err != nil {
		return nil
	}
	var idsToDelete []uint
	for _, s := range serverSaves {
		if s.Slot == slot {
			idsToDelete = append(idsToDelete, s.ID)
		}
	}
	if len(idsToDelete) == 0 {
		return nil
	}
	if err := a.rommSrv.DeleteServerSaves(idsToDelete); err != nil {
		return fmt.Errorf("failed to delete server saves: %w", err)
	}
	return nil
}

func removeSlotFromList(list []string, target string) []string {
	var updated []string
	for _, item := range list {
		if item != target {
			updated = append(updated, item)
		}
	}
	return updated
}

// DeleteSaveSlot deletes a save slot and all its server saves from RomM.
func (a *App) DeleteSaveSlot(id uint, slot string) error {
	slot = strings.TrimSpace(slot)
	if slot == "" {
		return fmt.Errorf("the legacy save slot cannot be deleted")
	}
	if err := a.deleteServerSavesForSlot(id, slot); err != nil {
		return err
	}
	key := strconv.FormatUint(uint64(id), 10)
	return a.configManager.Update(func(cfg *types.AppConfig) {
		if cfg.CustomSaveSlots != nil {
			if list, ok := cfg.CustomSaveSlots[key]; ok {
				cfg.CustomSaveSlots[key] = removeSlotFromList(list, slot)
			}
		}
		if cfg.GameSaveSlots != nil && cfg.GameSaveSlots[key] == slot {
			cfg.GameSaveSlots[key] = "default"
		}
	})
}

func isIgnoredStartupFile(name string) bool {
	if name == "" || strings.HasPrefix(name, ".") {
		return true
	}
	lower := strings.ToLower(name)
	return lower == "metadata.json" || (strings.HasPrefix(lower, "metadata_") && strings.HasSuffix(lower, ".json"))
}

func sortStartupFiles(files []string) {
	rank := func(f string) int {
		switch strings.ToLower(filepath.Ext(f)) {
		case constants.ExtM3u:
			return 0
		case constants.ExtCue:
			return 1
		default:
			return 2
		}
	}
	sort.SliceStable(files, func(i, j int) bool {
		rI, rJ := rank(files[i]), rank(files[j])
		if rI != rJ {
			return rI < rJ
		}
		return files[i] < files[j]
	})
}

// GetRomStartupFiles returns all available ROM files for starting the game.
func (a *App) GetRomStartupFiles(id uint) ([]string, error) {
	game, err := a.GetRom(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get ROM info: %w", err)
	}

	seen := make(map[string]bool)
	var filesList []string

	addFile := func(name string) {
		base := filepath.Base(name)
		if isIgnoredStartupFile(base) || seen[base] {
			return
		}
		seen[base] = true
		filesList = append(filesList, base)
	}

	a.collectDiskStartupFiles(&game, addFile)
	if len(filesList) == 0 {
		a.collectMetadataStartupFiles(&game, addFile)
	}

	sortStartupFiles(filesList)
	return filesList, nil
}

func (a *App) collectDiskStartupFiles(game *types.Game, addFile func(string)) {
	romDir := a.librarySrv.GetRomDir(game)
	entries, err := os.ReadDir(romDir)
	if err != nil {
		return
	}
	filtered := entries
	if a.configManager.GetConfig().UsePlatformFolder {
		filtered = library.FilterPlatformFolderFiles(entries, game)
	}
	for _, entry := range filtered {
		if !entry.IsDir() {
			addFile(entry.Name())
		}
	}
}

func (a *App) collectMetadataStartupFiles(game *types.Game, addFile func(string)) {
	for _, f := range game.Files {
		if f.FileName != "" {
			addFile(f.FileName)
		} else if f.FilePath != "" {
			addFile(f.FilePath)
		}
	}

	if game.FSName != "" {
		addFile(game.FSName)
	} else if game.FullPath != "" {
		addFile(game.FullPath)
	}
}

// --- Internal Provider Implementations ---

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
	return a.firmwareSrv.GetBiosDir()
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

func (a *App) ToggleUsePlatformFolder() (bool, error) {
	currentConfig := a.configManager.GetConfig()
	targetState := !currentConfig.UsePlatformFolder

	// Run migration
	if err := a.librarySrv.MigrateLibrary(targetState); err != nil {
		a.LogErrorf("Failed to migrate library: %v", err)
		return currentConfig.UsePlatformFolder, err
	}

	// Save new state
	if err := a.configManager.Update(func(cfg *types.AppConfig) {
		cfg.UsePlatformFolder = targetState
	}); err != nil {
		a.LogErrorf("Failed to update config during ToggleUsePlatformFolder: %v", err)
		return currentConfig.UsePlatformFolder, err
	}

	a.LogInfof("ToggleUsePlatformFolder: Successfully changed UsePlatformFolder to %t", targetState)
	return targetState, nil
}

func (a *App) ScanOrphanedRoms() ([]string, error) {
	return a.librarySrv.ScanOrphanedRoms()
}

func (a *App) DeleteOrphanedRoms(files []string) (int, error) {
	return a.librarySrv.DeleteOrphanedRoms(files)
}

// Lifecycle
func (a *App) Quit() {
	wailsRuntime.Quit(a.ctx)
}
