package sync

import (
	"fmt"
	"go-romm-sync/types"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
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

// Service manages the synchronization of saves and states.
type Service struct {
	library LibraryProvider
	romm    RomMProvider
}

// New creates a new Sync service.
func New(lib LibraryProvider, romm RomMProvider) *Service {
	return &Service{
		library: lib,
		romm:    romm,
	}
}

// GetSaves returns a list of local save files for a game.
func (s *Service) GetSaves(id uint) ([]types.FileItem, error) {
	return s.getGameFiles(id, "saves")
}

// GetStates returns a list of local state files for a game.
func (s *Service) GetStates(id uint) ([]types.FileItem, error) {
	return s.getGameFiles(id, "states")
}

func (s *Service) getGameFiles(id uint, subDir string) ([]types.FileItem, error) {
	game, err := s.romm.GetRom(id)
	if err != nil {
		return nil, err
	}

	romDir := s.library.GetRomDir(&game)
	dirPath := filepath.Join(romDir, subDir)

	items := []types.FileItem{}
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return items, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			coreName := entry.Name()
			coreDir := filepath.Join(dirPath, coreName)
			files, err := os.ReadDir(coreDir)
			if err != nil {
				continue
			}
			for _, f := range files {
				if !f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
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
			}
		}
	}
	return items, nil
}

// UploadSave reads a local save file and uploads it to RomM.
func (s *Service) UploadSave(id uint, core, filename string) error {
	return s.uploadServerAsset(id, core, filename, "saves")
}

// UploadState reads a local save state file and uploads it to RomM.
func (s *Service) UploadState(id uint, core, filename string) error {
	return s.uploadServerAsset(id, core, filename, "states")
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

	err = nil
	if subDir == "saves" {
		err = s.romm.RomMUploadSave(id, core, filename, content)
	} else {
		err = s.romm.RomMUploadState(id, core, filename, content)
	}

	if err != nil {
		return err
	}

	// Update local file time after successful upload to align with server
	now := time.Now()
	_ = os.Chtimes(cleanPath, now, now)

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
func (s *Service) DownloadServerSave(gameID uint, filePath string, core string, filename string, updatedAt string) error {
	return s.downloadServerAsset(gameID, filePath, core, filename, updatedAt, "saves")
}

// DownloadServerState downloads a state from RomM.
func (s *Service) DownloadServerState(gameID uint, filePath string, core string, filename string, updatedAt string) error {
	return s.downloadServerAsset(gameID, filePath, core, filename, updatedAt, "states")
}

func (s *Service) downloadServerAsset(gameID uint, filePath string, core string, filename string, updatedAt string, subDir string) error {
	game, err := s.romm.GetRom(gameID)
	if err != nil {
		return fmt.Errorf("failed to get ROM info: %w", err)
	}

	var reader io.ReadCloser
	var serverFilename string
	if subDir == "saves" {
		reader, serverFilename, err = s.romm.RomMDownloadSave(filePath)
	} else {
		reader, serverFilename, err = s.romm.RomMDownloadState(filePath)
	}

	if err != nil {
		return fmt.Errorf("failed to download %s from server: %w", subDir, err)
	}
	defer reader.Close()

	if filename == "" {
		filename = serverFilename
	}

	core, filename, err = s.ValidateAssetPath(core, filename)
	if err != nil {
		return err
	}

	romDir := s.library.GetRomDir(&game)
	baseDir := filepath.Join(romDir, subDir)
	destDir := filepath.Join(baseDir, core)

	rel, err := filepath.Rel(baseDir, destDir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("invalid path traversal detected")
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	destPath := filepath.Join(destDir, filename)
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create local %s file: %w", subDir, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, reader); err != nil {
		return fmt.Errorf("failed to write local %s file: %w", subDir, err)
	}

	out.Close() // Close before Chtimes

	if updatedAt != "" {
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			os.Chtimes(destPath, t, t)
		}
	}

	return nil
}

// ValidateAssetPath sanitizes the core and filename.
func (s *Service) ValidateAssetPath(core, filename string) (string, string, error) {
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
