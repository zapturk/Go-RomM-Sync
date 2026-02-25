package sync

import (
	"fmt"
	"go-romm-sync/types"
	"go-romm-sync/utils"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-romm-sync/constants"
	"go-romm-sync/utils/fileio"
)

// LibraryProvider defines the local library interactions needed for syncing.
type LibraryProvider interface {
	GetRomDir(game *types.Game) string
}

// RomMProvider defines the RomM API interactions needed for syncing.
type RomMProvider interface {
	GetRom(id uint) (types.Game, error)
	RomMUploadSave(id uint, core, filename string, content []byte) error
	RomMUploadState(id uint, core, filename string, content []byte) error
	RomMDownloadSave(filePath string) (io.ReadCloser, string, error)
	RomMDownloadState(filePath string) (io.ReadCloser, string, error)
}

// UIProvider defines logging and event emission.
type UIProvider interface {
	LogInfof(format string, args ...interface{})
	LogErrorf(format string, args ...interface{})
	EventsEmit(eventName string, args ...interface{})
}

// Service manages the synchronization of saves and states.
type Service struct {
	library LibraryProvider
	romm    RomMProvider
	ui      UIProvider
}

// New creates a new Sync service.
func New(lib LibraryProvider, romm RomMProvider, ui UIProvider) *Service {
	return &Service{
		library: lib,
		romm:    romm,
		ui:      ui,
	}
}

// GetSaves returns a list of local save files for a game.
func (s *Service) GetSaves(id uint) (items []types.FileItem, err error) {
	return s.getGameFiles(id, constants.DirSaves)
}

// GetStates returns a list of local state files for a game.
func (s *Service) GetStates(id uint) (items []types.FileItem, err error) {
	return s.getGameFiles(id, constants.DirStates)
}

func (s *Service) getGameFiles(id uint, subDir string) (items []types.FileItem, err error) {
	game, err := s.romm.GetRom(id)
	if err != nil {
		return nil, err
	}

	dirPath := filepath.Join(s.library.GetRomDir(&game), subDir)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return s.handleGetFilesError(err)
	}

	return s.collectCoreFiles(dirPath, entries), nil
}

func (s *Service) handleGetFilesError(err error) ([]types.FileItem, error) {
	if os.IsNotExist(err) {
		return []types.FileItem{}, nil
	}
	return nil, err
}

func (s *Service) collectCoreFiles(dirPath string, entries []os.DirEntry) []types.FileItem {
	var items []types.FileItem
	for _, entry := range entries {
		if entry.IsDir() {
			items = append(items, s.scanCoreDir(dirPath, entry.Name())...)
		}
	}
	return items
}

func (s *Service) scanCoreDir(dirPath, coreName string) []types.FileItem {
	coreDir := filepath.Join(dirPath, coreName)
	if coreName == "dolphin-emu" {
		return s.scanDolphinFiles(coreDir)
	}
	return s.scanFlatCoreFiles(coreName, coreDir)
}

func (s *Service) scanDolphinFiles(coreDir string) []types.FileItem {
	var items []types.FileItem
	gcDir := filepath.Join(coreDir, "User", "GC")
	for _, region := range []string{"USA", "EUR", "JPN"} {
		cardDir := filepath.Join(gcDir, region, "Card A")
		relCore := filepath.Join("dolphin-emu", "User", "GC", region, "Card A")
		items = append(items, s.scanFlatCoreFiles(relCore, cardDir)...)
	}
	return items
}

func (s *Service) scanFlatCoreFiles(coreName, coreDir string) []types.FileItem {
	var items []types.FileItem
	files, err := os.ReadDir(coreDir)
	if err != nil {
		return items
	}
	for _, f := range files {
		if f.IsDir() || strings.HasPrefix(f.Name(), ".") {
			continue
		}
		info, err := f.Info()
		updatedAt := ""
		if err == nil {
			updatedAt = info.ModTime().UTC().Format(time.RFC3339)
		}
		items = append(items, types.FileItem{
			Name:      f.Name(),
			Core:      coreName,
			UpdatedAt: updatedAt,
		})
	}
	return items
}

// UploadSave reads a local save file and uploads it to RomM.
func (s *Service) UploadSave(id uint, core, filename string) error {
	return s.uploadServerAsset(id, core, filename, constants.DirSaves)
}

// UploadState reads a local save state file and uploads it to RomM.
func (s *Service) UploadState(id uint, core, filename string) error {
	return s.uploadServerAsset(id, core, filename, constants.DirStates)
}

func (s *Service) uploadServerAsset(id uint, core, filename, subDir string) error {
	game, err := s.romm.GetRom(id)
	if err != nil {
		return fmt.Errorf("failed to get ROM info: %w", err)
	}

	romDir := s.library.GetRomDir(&game)
	baseDir := filepath.Join(romDir, subDir)
	filePath := filepath.Join(baseDir, core, filename)

	cleanPath := filepath.Clean(filePath)
	cleanBase := filepath.Clean(baseDir)

	rel, err := filepath.Rel(cleanBase, cleanPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("invalid path traversal detected")
	}

	content, err := os.ReadFile(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to read local %s file: %w", subDir, err)
	}

	if subDir == constants.DirSaves {
		err = s.romm.RomMUploadSave(id, core, filename, content)
	} else {
		err = s.romm.RomMUploadState(id, core, filename, content)
	}

	if err != nil {
		return err
	}

	// Update local file time after successful upload to align with server
	now := time.Now()
	if err := os.Chtimes(cleanPath, now, now); err != nil {
		s.ui.LogErrorf("uploadServerAsset: Failed to update local file time: %v", err)
	}

	return nil
}

// DeleteGameFile deletes a local save or state file.
func (s *Service) DeleteGameFile(id uint, subDir, core, filename string) error {
	game, err := s.romm.GetRom(id)
	if err != nil {
		return err
	}

	romDir := s.library.GetRomDir(&game)
	baseDir := filepath.Join(romDir, subDir)
	filePath := filepath.Join(baseDir, core, filename)

	cleanPath := filepath.Clean(filePath)
	cleanBase := filepath.Clean(baseDir)

	rel, err := filepath.Rel(cleanBase, cleanPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("invalid path traversal detected")
	}

	_, err = os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to access file %s: %w", cleanPath, err)
	}

	err = os.Remove(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to delete file %s: %w", cleanPath, err)
	}
	return nil
}

// DownloadServerSave downloads a save from RomM.
func (s *Service) DownloadServerSave(gameID uint, filePath, core, filename, updatedAt string) error {
	return s.downloadServerAsset(gameID, filePath, core, filename, updatedAt, constants.DirSaves)
}

// DownloadServerState downloads a state from RomM.
func (s *Service) DownloadServerState(gameID uint, filePath, core, filename, updatedAt string) error {
	return s.downloadServerAsset(gameID, filePath, core, filename, updatedAt, constants.DirStates)
}

func (s *Service) downloadServerAsset(gameID uint, filePath, core, filename, updatedAt, subDir string) error {
	game, err := s.romm.GetRom(gameID)
	if err != nil {
		return fmt.Errorf("failed to get ROM info: %w", err)
	}

	var reader io.ReadCloser
	var serverFilename string
	if subDir == constants.DirSaves {
		reader, serverFilename, err = s.romm.RomMDownloadSave(filePath)
	} else {
		reader, serverFilename, err = s.romm.RomMDownloadState(filePath)
	}

	if err != nil {
		return fmt.Errorf("failed to download %s from server: %w", subDir, err)
	}
	defer fileio.Close(reader, nil, "downloadServerAsset: Failed to close reader")

	if filename == "" {
		filename = serverFilename
	}

	destPath, err := s.prepareAssetPath(&game, core, filename, subDir)
	if err != nil {
		return err
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create local %s file: %w", subDir, err)
	}
	defer fileio.Close(out, nil, "downloadServerAsset: Failed to close output file")

	if _, err := io.Copy(out, reader); err != nil {
		return fmt.Errorf("failed to write local %s file: %w", subDir, err)
	}

	if updatedAt != "" {
		s.setFileTime(destPath, updatedAt)
	}

	return nil
}

func (s *Service) prepareAssetPath(game *types.Game, core, filename, subDir string) (string, error) {
	core, filename, err := s.ValidateAssetPath(core, filename)
	if err != nil {
		return "", err
	}

	romDir := s.library.GetRomDir(game)
	baseDir := filepath.Join(romDir, subDir)

	// Remap the Dolphin "Card A" / "Card B" emulator names from RomM to the correct
	// local nested path that the dolphin-emu RetroArch core expects.
	// RomM stores these saves with emulator = "Card A", but locally they must live at:
	//   saves/dolphin-emu/User/GC/{region}/Card A/
	// We default to USA region; the file will be placed correctly for NTSC-U games.
	switch core {
	case "Card A":
		core = filepath.Join("dolphin-emu", "User", "GC", "USA", "Card A")
	case "Card B":
		core = filepath.Join("dolphin-emu", "User", "GC", "USA", "Card B")
	}

	destDir := filepath.Join(baseDir, core)
	rel, err := filepath.Rel(baseDir, destDir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("invalid path traversal detected")
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create destination directory: %w", err)
	}

	return filepath.Join(destDir, filename), nil
}

func (s *Service) setFileTime(destPath, updatedAt string) {
	t, err := utils.ParseTimestamp(updatedAt)
	if err != nil {
		return
	}
	if err := os.Chtimes(destPath, t, t); err != nil {
		s.ui.LogErrorf("setFileTime: Failed to update local file time for %s: %v", destPath, err)
	}
}

// ValidateAssetPath sanitizes the core and filename.
func (s *Service) ValidateAssetPath(core, filename string) (coreBase, fileBase string, err error) {
	core = filepath.Base(filepath.Clean(core))
	if core == "." || core == ".." {
		return "", "", fmt.Errorf("invalid core name")
	}

	filename = filepath.Base(filepath.Clean(filename))
	if filename == "." || filename == ".." {
		return "", "", fmt.Errorf("invalid filename")
	}

	return core, filename, nil
}
