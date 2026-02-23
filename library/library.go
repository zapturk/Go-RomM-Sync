package library

import (
	"fmt"
	"go-romm-sync/retroarch"
	"go-romm-sync/types"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ConfigProvider defines the configuration needed for library management.
type ConfigProvider interface {
	GetLibraryPath() string
	SaveDefaultLibraryPath(path string) error
}

// RomMProvider defines the RomM API interactions needed for library management.
type RomMProvider interface {
	DownloadFile(game *types.Game) (io.ReadCloser, string, error)
	GetRom(id uint) (types.Game, error)
}

// UIProvider defines logging and event emission.
type UIProvider interface {
	LogInfof(format string, args ...interface{})
	LogErrorf(format string, args ...interface{})
	EventsEmit(eventName string, args ...interface{})
}

type ProgressWriter struct {
	Total      int64
	Downloaded int64
	GameID     uint
	UI         UIProvider
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.Downloaded += int64(n)
	if pw.Total > 0 {
		percentage := float64(pw.Downloaded) / float64(pw.Total) * 100
		pw.UI.EventsEmit("download-progress", map[string]interface{}{
			"game_id":    pw.GameID,
			"percentage": percentage,
		})
	}
	return n, nil
}

// Service manages the local ROM library.
type Service struct {
	config ConfigProvider
	romm   RomMProvider
	ui     UIProvider
}

// New creates a new Library service.
func New(cfg ConfigProvider, romm RomMProvider, ui UIProvider) *Service {
	return &Service{
		config: cfg,
		romm:   romm,
		ui:     ui,
	}
}

// GetRomDir returns the local directory where a ROM is stored.
func (s *Service) GetRomDir(game *types.Game) string {
	libPath := s.config.GetLibraryPath()
	return filepath.Join(libPath, filepath.Dir(game.FullPath), fmt.Sprintf("%d", game.ID))
}

// DownloadRomToLibrary downloads a ROM directly to the configured library path.
func (s *Service) DownloadRomToLibrary(id uint) error {
	libPath := s.config.GetLibraryPath()
	if libPath == "" {
		// This is a bit tricky as the original logic tried to get a default path.
		// We'll assume the caller handles default path logic or we provide a way to save it.
		return fmt.Errorf("library path is not configured")
	}

	game, err := s.romm.GetRom(id)
	if err != nil {
		return fmt.Errorf("failed to get ROM info: %w", err)
	}

	reader, _, err := s.romm.DownloadFile(&game)
	if err != nil {
		return err
	}
	defer reader.Close()

	destDir := s.GetRomDir(&game)
	filename := filepath.Base(game.FullPath)
	destPath := filepath.Join(destDir, filename)

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer out.Close()

	pw := &ProgressWriter{
		Total:  game.FileSize,
		GameID: game.ID,
		UI:     s.ui,
	}

	if _, err := io.Copy(io.MultiWriter(out, pw), reader); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}

// GetRomDownloadStatus checks if a ROM has been downloaded.
func (s *Service) GetRomDownloadStatus(id uint) (bool, error) {
	libPath := s.config.GetLibraryPath()
	if libPath == "" {
		return false, nil
	}

	game, err := s.romm.GetRom(id)
	if err != nil {
		return false, nil
	}

	romDir := s.GetRomDir(&game)
	if info, err := os.Stat(romDir); err == nil && info.IsDir() {
		return s.findRomPath(romDir) != "", nil
	}

	return false, nil
}

// findRomPath looks for a valid ROM file in the given directory.
func (s *Service) findRomPath(romDir string) string {
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
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if _, ok := retroarch.CoreMap[ext]; ok || ext == ".zip" {
			return filepath.Join(romDir, name)
		}
	}
	return ""
}

// DeleteRom removes a downloaded ROM.
func (s *Service) DeleteRom(id uint) error {
	libPath := s.config.GetLibraryPath()
	if libPath == "" {
		return fmt.Errorf("library path is not configured")
	}

	game, err := s.romm.GetRom(id)
	if err != nil {
		return fmt.Errorf("failed to get ROM info for deletion: %w", err)
	}

	romDir := s.GetRomDir(&game)
	if _, err := os.Stat(romDir); err == nil {
		if err := os.RemoveAll(romDir); err != nil {
			s.ui.LogErrorf("DeleteRom: Error during RemoveAll for ID %d: %v", id, err)
			return fmt.Errorf("failed to delete ROM directory: %w", err)
		}
		s.ui.LogInfof("DeleteRom: Successfully deleted ROM %d from library", id)
	}

	return nil
}

// FindRomPath is a public wrapper for finding a ROM path.
func (s *Service) FindRomPath(romDir string) string {
	return s.findRomPath(romDir)
}
