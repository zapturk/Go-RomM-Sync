package launcher

import (
	"context"
	"fmt"
	"go-romm-sync/retroarch"
	"go-romm-sync/types"
	"go-romm-sync/utils"
	"os"
	"path/filepath"
	"strings"
)

// ConfigProvider defines the configuration needed for launching games.
type ConfigProvider interface {
	GetLibraryPath() string
	GetRetroArchPath() string
	GetCheevosCredentials() (string, string)
	GetBiosDir() string
}

// RomMProvider defines the RomM API interactions needed for launching games.
type RomMProvider interface {
	GetRom(id uint) (types.Game, error)
}

// PreferenceProvider defines the interface for saving user preferences.
type PreferenceProvider interface {
	SaveLastUsedCore(platformSlug, coreName string) error
	GetResolvedPlatformSlug(game *types.Game) string
}

// UIProvider defines the UI interactions needed for launching games.
type UIProvider interface {
	SelectRetroArchExecutable() (string, error)
	LogInfof(format string, args ...interface{})
	LogErrorf(format string, args ...interface{})
	EventsEmit(eventName string, args ...interface{})
	WindowHide()
	WindowShow()
	WindowUnminimise()
	WindowSetAlwaysOnTop(b bool)
}

// Launcher handles the orchestration of launching a game.
type Launcher struct {
	config ConfigProvider
	romm   RomMProvider
	prefs  PreferenceProvider
	ui     UIProvider
	ctx    context.Context
}

// New creates a new Launcher.
func New(cfg ConfigProvider, romm RomMProvider, prefs PreferenceProvider, ui UIProvider) *Launcher {
	return &Launcher{
		config: cfg,
		romm:   romm,
		prefs:  prefs,
		ui:     ui,
	}
}

// SetContext sets the Wails context for the launcher.
func (l *Launcher) SetContext(ctx context.Context) {
	l.ctx = ctx
}

// PlayRom attempts to launch the given ROM.
func (l *Launcher) PlayRom(id uint) error {
	libPath := l.config.GetLibraryPath()
	if libPath == "" {
		return fmt.Errorf("library path is not configured")
	}

	game, err := l.romm.GetRom(id)
	if err != nil {
		return fmt.Errorf("failed to get ROM info: %w", err)
	}
	// 2. Find local ROM path
	relDir := utils.SanitizePath(filepath.Dir(game.FullPath))
	romDir := filepath.Join(libPath, relDir, fmt.Sprintf("%d", game.ID))
	romPath := l.findRomPath(&game, romDir)
	if romPath == "" {
		return fmt.Errorf("no valid ROM file found in %s, please download it first", romDir)
	}

	// 3. Check if RetroArch is Configured
	exePath := l.config.GetRetroArchPath()
	if exePath == "" {
		// Prompt user manually if they haven't set it yet
		exePath, err = l.ui.SelectRetroArchExecutable()
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
	cheevosUser, cheevosPass := l.config.GetCheevosCredentials()

	// Resolve and save core preference before launching
	platformSlug := l.prefs.GetResolvedPlatformSlug(&game)
	cores := retroarch.GetCoresForPlatform(platformSlug)
	if len(cores) > 0 && platformSlug != "" {
		_ = l.prefs.SaveLastUsedCore(platformSlug, cores[0])
	}

	// Delegate UI lifecycle to launch helper inside retroarch/manager.go (which handles hiding window, etc.)
	err = retroarch.Launch(l.ui, exePath, romPath, cheevosUser, cheevosPass, "", platformSlug, l.config.GetBiosDir())
	if err != nil {
		return fmt.Errorf("failed to launch game: %w", err)
	}

	return nil
}

// PlayRomWithCore is like PlayRom but lets the caller specify the libretro core
// base name (e.g. "snes9x_libretro") to use instead of the auto-detected default.
func (l *Launcher) PlayRomWithCore(id uint, coreOverride string) error {
	libPath := l.config.GetLibraryPath()
	if libPath == "" {
		return fmt.Errorf("library path is not configured")
	}

	game, err := l.romm.GetRom(id)
	if err != nil {
		return fmt.Errorf("failed to get ROM info: %w", err)
	}

	relDir := utils.SanitizePath(filepath.Dir(game.FullPath))
	romDir := filepath.Join(libPath, relDir, fmt.Sprintf("%d", game.ID))
	romPath := l.findRomPath(&game, romDir)
	if romPath == "" {
		return fmt.Errorf("no valid ROM file found in %s, please download it first", romDir)
	}

	exePath := l.config.GetRetroArchPath()
	if exePath == "" {
		exePath, err = l.ui.SelectRetroArchExecutable()
		if err != nil {
			return fmt.Errorf("retroarch not configured: %w", err)
		}
		if exePath == "" {
			return fmt.Errorf("launch cancelled: RetroArch executable not selected")
		}
	} else {
		if _, err := os.Stat(exePath); err != nil {
			return fmt.Errorf("retroarch executable not found at configured path: %s", exePath)
		}
	}

	// Save preference before launching
	platformSlug := l.prefs.GetResolvedPlatformSlug(&game)
	coreToSave := coreOverride
	if coreToSave == "" {
		cores := retroarch.GetCoresForPlatform(platformSlug)
		if len(cores) > 0 {
			coreToSave = cores[0]
		}
	}
	if coreToSave != "" && platformSlug != "" {
		_ = l.prefs.SaveLastUsedCore(platformSlug, coreToSave)
	}

	cheevosUser, cheevosPass := l.config.GetCheevosCredentials()
	err = retroarch.Launch(l.ui, exePath, romPath, cheevosUser, cheevosPass, coreOverride, platformSlug, l.config.GetBiosDir())
	if err != nil {
		return fmt.Errorf("failed to launch game: %w", err)
	}

	return nil
}

// findRomPath looks for a valid ROM file in the given directory.
func (l *Launcher) findRomPath(game *types.Game, romDir string) string {
	files, err := os.ReadDir(romDir)
	if err != nil {
		return ""
	}

	if p := l.findCueFile(romDir, files); p != "" {
		return p
	}

	if p := l.findExactMatch(game, romDir); p != "" {
		return p
	}

	if p := l.findPlatformPreferredRom(game, romDir, files); p != "" {
		return p
	}

	return l.findAnyRom(romDir, files)
}

func (l *Launcher) findCueFile(romDir string, files []os.DirEntry) string {
	for _, file := range files {
		if !file.IsDir() && strings.ToLower(filepath.Ext(file.Name())) == ".cue" {
			return filepath.Join(romDir, file.Name())
		}
	}
	return ""
}

func (l *Launcher) findExactMatch(game *types.Game, romDir string) string {
	baseName := filepath.Base(game.FullPath)
	directPath := filepath.Join(romDir, baseName)
	if info, err := os.Stat(directPath); err == nil && !info.IsDir() {
		return directPath
	}
	return ""
}

func (l *Launcher) findPlatformPreferredRom(game *types.Game, romDir string, files []os.DirEntry) string {
	platformCores := retroarch.GetCoresForPlatform(game.Platform.Slug)
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

func (l *Launcher) findAnyRom(romDir string, files []os.DirEntry) string {
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
