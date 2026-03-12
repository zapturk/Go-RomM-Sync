package library

import (
	"encoding/json"
	"fmt"
	"go-romm-sync/constants"
	"go-romm-sync/retroarch"
	"go-romm-sync/types"
	"go-romm-sync/utils"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go-romm-sync/utils/archive"
	"go-romm-sync/utils/fileio"
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
	GetFirmware(platformID uint) ([]types.Firmware, error)
	DownloadFirmwareContent(id uint, fileName string) (io.ReadCloser, string, error)
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
	relPath := utils.SanitizePath(filepath.Dir(game.FullPath))
	return filepath.Join(libPath, relPath, fmt.Sprintf("%d", game.ID))
}

// GetBiosDir returns the local directory where BIOS files are stored globally in the library.
func (s *Service) GetBiosDir() string {
	return filepath.Join(s.config.GetLibraryPath(), constants.DirBios)
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
	defer fileio.Close(reader, nil, "DownloadRomToLibrary: Failed to close reader")

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
	defer fileio.Close(out, s.ui.LogErrorf, "DownloadRomToLibrary: Failed to close destination file")

	pw := &ProgressWriter{
		Total:  game.FileSize,
		GameID: game.ID,
		UI:     s.ui,
	}

	if _, err := io.Copy(io.MultiWriter(out, pw), reader); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	// Archive check: Extract .cue/.bin if present
	s.ui.EventsEmit("library-status", map[string]interface{}{"game_id": id, "status": "extracting"})
	if extracted, err := archive.ExtractCueBin(destPath, destDir); err != nil {
		s.ui.LogErrorf("DownloadRomToLibrary: Extraction failed for %s: %v", destPath, err)
	} else if extracted {
		s.ui.LogInfof("DownloadRomToLibrary: Extracted .cue/.bin files from archive: %s", destPath)
		// Optionally delete the original archive to keep the folder clean
		if err := os.Remove(destPath); err != nil {
			s.ui.LogErrorf("DownloadRomToLibrary: Failed to remove original archive %s: %v", destPath, err)
		}
	}

	// Save metadata for offline use
	if err := s.SaveMetadata(game); err != nil {
		s.ui.LogErrorf("DownloadRomToLibrary: Failed to save metadata for ID %d: %v", id, err)
	}

	return nil
}

// SaveMetadata saves the game metadata to a local JSON file.
func (s *Service) SaveMetadata(game types.Game) error {
	destDir := s.GetRomDir(&game)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	metadataPath := filepath.Join(destDir, "metadata.json")
	data, err := json.MarshalIndent(game, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	return os.WriteFile(metadataPath, data, 0o644)
}

// GetLocalLibrary scans the library directory and returns a list of games with metadata.
func (s *Service) GetLocalLibrary(limit, offset, platformID int, search string) ([]types.Game, int, error) {
	libPath := s.config.GetLibraryPath()
	if libPath == "" {
		return nil, 0, fmt.Errorf("library path not configured")
	}

	var games []types.Game
	err := filepath.Walk(libPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}

		metadataPath := filepath.Join(path, "metadata.json")
		if _, err := os.Stat(metadataPath); err == nil {
			data, err := os.ReadFile(metadataPath)
			if err != nil {
				return nil
			}

			var game types.Game
			if err := json.Unmarshal(data, &game); err != nil {
				return nil
			}

			// Filter by platform
			if platformID != 0 && int(game.PlatformID) != platformID {
				return nil
			}

			// Filter by search
			if search != "" {
				searchLower := strings.ToLower(search)
				if !strings.Contains(strings.ToLower(game.Title), searchLower) {
					return nil
				}
			}

			games = append(games, game)
		}
		return nil
	})

	if err != nil {
		return nil, 0, err
	}

	total := len(games)
	start := offset
	if start > total {
		start = total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return games[start:end], total, nil
}

// GetLocalGame retrieves local metadata for a specific game ID.
func (s *Service) GetLocalGame(id uint) (types.Game, error) {
	libPath := s.config.GetLibraryPath()
	var foundGame types.Game
	var found bool

	err := filepath.Walk(libPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || found || !info.IsDir() {
			return nil
		}

		if filepath.Base(path) == fmt.Sprintf("%d", id) {
			metadataPath := filepath.Join(path, "metadata.json")
			if _, err := os.Stat(metadataPath); err == nil {
				data, err := os.ReadFile(metadataPath)
				if err != nil {
					return nil
				}
				if err := json.Unmarshal(data, &foundGame); err == nil {
					found = true
				}
			}
		}
		return nil
	})

	if err != nil {
		return types.Game{}, err
	}
	if !found {
		return types.Game{}, fmt.Errorf("game %d not found in local library", id)
	}

	return foundGame, nil
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

// DownloadFirmware downloads a firmware file to the library's bios directory.
func (s *Service) DownloadFirmware(fw *types.Firmware) error {
	biosDir := s.GetBiosDir()
	if err := os.MkdirAll(biosDir, 0o755); err != nil {
		return fmt.Errorf("failed to create bios directory: %w", err)
	}

	reader, filename, err := s.romm.DownloadFirmwareContent(fw.ID, fw.FileName)
	if err != nil {
		return err
	}
	defer fileio.Close(reader, nil, "DownloadFirmware: Failed to close reader")

	// Save to a temporary file first to check for archives and calculate MD5 if needed
	tempDir, err := os.MkdirTemp("", "go-romm-sync-bios-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	tempFile := filepath.Join(tempDir, filename)
	if err := fileio.WriteFileFromReader(tempFile, reader, 0o644); err != nil {
		return err
	}

	// Check if it's an archive
	extracted, err := archive.Extract(tempFile, tempDir)
	if err != nil {
		s.ui.LogErrorf("DownloadFirmware: Failed to extract BIOS archive: %v", err)
	}

	if extracted {
		s.ui.LogInfof("DownloadFirmware: Extracted BIOS archive %s", filename)
		// Process each extracted file
		return filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || path == tempFile {
				return nil
			}

			md5, err := fileio.GetMD5(path)
			if err != nil {
				s.ui.LogErrorf("DownloadFirmware: Failed to calculate MD5 for %s: %v", path, err)
				return nil
			}

			destFilename := filepath.Base(path)
			if mappedName := retroarch.GetBiosFilename(md5); mappedName != "" {
				s.ui.LogInfof("DownloadFirmware: Mapping extracted BIOS MD5 %s to canonical name %s (orig: %s)", md5, mappedName, destFilename)
				destFilename = mappedName
			}

			destPath := filepath.Join(biosDir, destFilename)
			// Move file to final destination
			return os.Rename(path, destPath)
		})
	}

	// Not an archive, process single file
	md5 := fw.MD5Hash
	if md5 == "" {
		var err error
		md5, err = fileio.GetMD5(tempFile)
		if err != nil {
			s.ui.LogErrorf("DownloadFirmware: Failed to calculate MD5 for %s: %v", filename, err)
		}
	}

	finalFilename := filename
	if mappedName := retroarch.GetBiosFilename(md5); mappedName != "" {
		s.ui.LogInfof("DownloadFirmware: Mapping BIOS MD5 %s to canonical name %s (orig: %s)", md5, mappedName, filename)
		finalFilename = mappedName
	}

	destPath := filepath.Join(biosDir, finalFilename)
	return os.Rename(tempFile, destPath)
}

// CleanupFirmware removes known canonical BIOS files from the bios directory for a specific platform.
// This is used when unsetting firmware for a platform to ensure RetroArch doesn't find them.
func (s *Service) CleanupFirmware(platformSlug string) error {
	biosDir := s.GetBiosDir()
	biosNames := retroarch.GetBiosFilenamesForPlatform(platformSlug)

	for _, name := range biosNames {
		path := filepath.Join(biosDir, name)
		if _, err := os.Stat(path); err == nil {
			s.ui.LogInfof("CleanupFirmware: Removing canonical BIOS file %s for platform %s", name, platformSlug)
			fileio.Remove(path, s.ui.LogErrorf)
		}
	}
	return nil
}
