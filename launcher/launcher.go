package launcher

import (
	"context"
	"fmt"
	"go-romm-sync/retroarch"
	"go-romm-sync/types"
	"os"
	"path/filepath"
	"strings"
)

// ConfigProvider defines the configuration needed for launching games.
type ConfigProvider interface {
	GetLibraryPath() string
	GetRetroArchPath() string
	GetCheevosCredentials() (string, string)
}

// RomMProvider defines the RomM API interactions needed for launching games.
type RomMProvider interface {
	GetRom(id uint) (types.Game, error)
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
	ui     UIProvider
	ctx    context.Context
}

// New creates a new Launcher.
func New(cfg ConfigProvider, romm RomMProvider, ui UIProvider) *Launcher {
	return &Launcher{
		config: cfg,
		romm:   romm,
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

	l.ui.LogInfof("PlayRom: Fetching game info for ID %d", id)
	game, err := l.romm.GetRom(id)
	if err != nil {
		return fmt.Errorf("failed to get ROM info: %w", err)
	}
	l.ui.LogInfof("PlayRom: Game info fetched. Name: %s, ID in struct: %d, FullPath: %s", game.Title, game.ID, game.FullPath)

	// 2. Find local ROM path
	romDir := filepath.Join(libPath, filepath.Dir(game.FullPath), fmt.Sprintf("%d", game.ID))
	l.ui.LogInfof("PlayRom: Calculated romDir: %s", romDir)
	romPath := l.findRomPath(romDir)
	l.ui.LogInfof("PlayRom: Found romPath: %s", romPath)
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

	// Delegate UI lifecycle to launch helper inside retroarch/manager.go (which handles hiding window, etc.)
	err = retroarch.Launch(l.ui, exePath, romPath, cheevosUser, cheevosPass)
	if err != nil {
		return fmt.Errorf("failed to launch game: %w", err)
	}

	return nil
}

// findRomPath looks for a valid ROM file in the given directory.
func (l *Launcher) findRomPath(romDir string) string {
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
			continue // Skip hidden files
		}
		ext := strings.ToLower(filepath.Ext(name))
		if _, ok := retroarch.CoreMap[ext]; ok || ext == ".zip" {
			return filepath.Join(romDir, name)
		}
	}
	return ""
}
