package library

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go-romm-sync/config"
	"go-romm-sync/constants"
	"go-romm-sync/retroarch"
	"go-romm-sync/rommsrv"
	"go-romm-sync/types"
	"go-romm-sync/utils"
	"go-romm-sync/utils/archive"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ponytail: nearly identical to retroarch.progressWriter (same io.Writer + percent-based event emission). Share one.
type ProgressWriter struct {
	Total       int64
	Downloaded  int64
	GameID      uint
	UI          types.UIProvider
	LastPercent float64
	LastEmit    time.Time
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.Downloaded += int64(n)
	if pw.Total > 0 {
		percentage := float64(pw.Downloaded) / float64(pw.Total) * 100
		// Throttle: emit if percentage changed significantly (>= 1%) OR it's been > 500ms
		if percentage-pw.LastPercent >= 1.0 || time.Since(pw.LastEmit) > 500*time.Millisecond || percentage >= 100 {
			pw.UI.EventsEmit("download-progress", map[string]interface{}{
				"game_id":    pw.GameID,
				"percentage": percentage,
			})
			pw.LastPercent = percentage
			pw.LastEmit = time.Now()
		}
	}
	return n, nil
}

// Service manages the local ROM library.
type Service struct {
	config *config.ConfigManager
	romm   *rommsrv.Service
	ui     types.UIProvider
}

// New creates a new Library service.
func New(cfg *config.ConfigManager, romm *rommsrv.Service, ui types.UIProvider) *Service {
	return &Service{
		config: cfg,
		romm:   romm,
		ui:     ui,
	}
}

// GetRomDir returns the local directory where a ROM is stored.
func (s *Service) GetRomDir(game *types.Game) string {
	libPath := ""
	usePlatformFolder := false
	if s.config != nil {
		libPath = s.config.GetConfig().LibraryPath
		usePlatformFolder = s.config.GetConfig().UsePlatformFolder
	}
	relPath := utils.SanitizePath(filepath.Dir(game.FullPath))
	if usePlatformFolder {
		return filepath.Join(libPath, relPath)
	}
	return filepath.Join(libPath, relPath, fmt.Sprintf("%d", game.ID))
}

// GetMetadataPath returns the path to the game's metadata file.
func (s *Service) GetMetadataPath(game *types.Game) string {
	destDir := s.GetRomDir(game)
	usePlatformFolder := false
	if s.config != nil {
		usePlatformFolder = s.config.GetConfig().UsePlatformFolder
	}
	if usePlatformFolder {
		return filepath.Join(destDir, fmt.Sprintf("metadata_%d.json", game.ID))
	}
	return filepath.Join(destDir, "metadata.json")
}

// DownloadRomToLibrary downloads a ROM directly to the configured library path.
func (s *Service) DownloadRomToLibrary(ctx context.Context, id uint) error {
	libPath := s.config.GetConfig().LibraryPath
	if libPath == "" {
		// This is a bit tricky as the original logic tried to get a default path.
		// We'll assume the caller handles default path logic or we provide a way to save it.
		return fmt.Errorf("library path is not configured")
	}

	game, err := s.romm.GetRom(id)
	if err != nil {
		return fmt.Errorf("failed to get ROM info: %w", err)
	}

	reader, _, err := s.romm.GetClient().DownloadFile(ctx, &game)
	if err != nil {
		return err
	}
	defer reader.Close() //nolint:errcheck

	destDir := s.GetRomDir(&game)
	filename := filepath.Base(game.FullPath)
	destPath := filepath.Join(destDir, filename)

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	var downloadSuccess bool
	defer func() {
		if !downloadSuccess {
			if _, err := os.Stat(destPath); err == nil {
				s.ui.LogInfof("DownloadRomToLibrary: Cleaning up partial/failed download at %s", destPath)
				_ = os.Remove(destPath) // Ignore error as it's just cleanup
			}
		}
	}()

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() {
		if err := out.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			s.ui.LogErrorf("DownloadRomToLibrary: Failed to close destination file: %v", err)
		}
	}()

	pw := &ProgressWriter{
		Total:    game.FileSize,
		GameID:   game.ID,
		UI:       s.ui,
		LastEmit: time.Now(),
	}

	s.ui.LogInfof("DownloadRomToLibrary: Starting download for ID %d, Size: %d", id, game.FileSize)
	if _, err := io.Copy(io.MultiWriter(out, pw), reader); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}
	downloadSuccess = true

	// Explicitly close the file handle so Windows allows extraction and deletion
	_ = out.Close()

	return s.postDownloadProcessing(id, &game, destPath, destDir)
}

func (s *Service) postDownloadProcessing(id uint, game *types.Game, destPath, destDir string) error {
	// Archive check: Extract .cue/.bin or GameCube files if present
	s.ui.EventsEmit("library-status", map[string]interface{}{"game_id": id, "status": "extracting"})

	var extracted bool
	var err error

	slug := game.Platform.Slug
	if slug == "" {
		slug = game.PlatformSlug
	}
	slug = strings.ToLower(slug)

	switch slug {
	case "ps2":
		extracted, err = archive.ExtractPS2(destPath, destDir)
		if err != nil {
			s.ui.LogErrorf("DownloadRomToLibrary: PS2 extraction failed for %s: %v", destPath, err)
		} else if extracted {
			s.ui.LogInfof("DownloadRomToLibrary: Extracted PS2 files from archive: %s", destPath)
		}
	case "gamecube":
		extracted, err = archive.ExtractGameCube(destPath, destDir)
		if err != nil {
			s.ui.LogErrorf("DownloadRomToLibrary: GameCube extraction failed for %s: %v", destPath, err)
		} else if extracted {
			s.ui.LogInfof("DownloadRomToLibrary: Extracted GameCube files from archive: %s", destPath)
		}
	default:
		extracted, err = archive.ExtractCueBin(destPath, destDir)
		if err != nil {
			s.ui.LogErrorf("DownloadRomToLibrary: .cue/.bin extraction failed for %s: %v", destPath, err)
		} else if extracted {
			s.ui.LogInfof("DownloadRomToLibrary: Extracted .cue/.bin files from archive: %s", destPath)
		}
	}

	// Archive cleanup: Remove the source archive only if something was extracted
	if extracted {
		if err := os.Remove(destPath); err != nil {
			s.ui.LogErrorf("DownloadRomToLibrary: Failed to remove archive after extraction: %v", err)
		}
	}

	// Save metadata
	if err := s.SaveMetadata(game); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	s.ui.EventsEmit("library-status", map[string]interface{}{"game_id": id, "status": "downloaded"})
	return nil
}

// SaveMetadata saves the game metadata to a local JSON file.
func (s *Service) SaveMetadata(game *types.Game) error {
	destDir := s.GetRomDir(game)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	metadataPath := s.GetMetadataPath(game)
	data, err := json.MarshalIndent(game, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	return os.WriteFile(metadataPath, data, 0o644)
}

// GetLocalLibrary scans the library directory and returns a list of games with metadata.
func (s *Service) GetLocalLibrary(limit, offset, platformID int, search string) ([]types.Game, int, error) {
	libPath := s.config.GetConfig().LibraryPath
	if libPath == "" {
		return nil, 0, fmt.Errorf("library path not configured")
	}

	var games []types.Game
	err := filepath.Walk(libPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		name := info.Name()
		isMetadata := name == "metadata.json" || (strings.HasPrefix(name, "metadata_") && strings.HasSuffix(name, ".json"))
		if isMetadata {
			data, err := os.ReadFile(path)
			if err != nil {
				s.ui.LogErrorf("GetLocalLibrary: Failed to read metadata at %s: %v", path, err)
				return nil
			}

			var game types.Game
			if err := json.Unmarshal(data, &game); err != nil {
				s.ui.LogErrorf("GetLocalLibrary: Failed to unmarshal metadata at %s: %v", path, err)
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
	libPath := s.config.GetConfig().LibraryPath
	var foundGame types.Game
	var found bool

	err := filepath.Walk(libPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || found || info.IsDir() {
			return nil
		}

		name := info.Name()
		if name == "metadata.json" {
			if filepath.Base(filepath.Dir(path)) == fmt.Sprintf("%d", id) {
				data, err := os.ReadFile(path)
				if err == nil {
					if err := json.Unmarshal(data, &foundGame); err == nil && foundGame.ID == id {
						found = true
					}
				}
			}
		} else if name == fmt.Sprintf("metadata_%d.json", id) {
			data, err := os.ReadFile(path)
			if err == nil {
				if err := json.Unmarshal(data, &foundGame); err == nil && foundGame.ID == id {
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
	libPath := s.config.GetConfig().LibraryPath
	if libPath == "" {
		return false, nil
	}

	game, err := s.romm.GetRom(id)
	if err != nil {
		return false, nil
	}

	romDir := s.GetRomDir(&game)
	if info, err := os.Stat(romDir); err == nil && info.IsDir() {
		return s.findRomPath(romDir, &game) != "", nil
	}

	return false, nil
}

// findRomPath looks for a valid ROM file in the given directory.
func (s *Service) findRomPath(romDir string, game *types.Game) string {
	files, err := os.ReadDir(romDir)
	if err != nil {
		return ""
	}

	var filtered []os.DirEntry
	usePlatformFolder := false
	if s.config != nil {
		usePlatformFolder = s.config.GetConfig().UsePlatformFolder
	}
	if usePlatformFolder && game != nil {
		expectedBase := filepath.Base(game.FullPath)
		expectedNameWithoutExt := strings.TrimSuffix(expectedBase, filepath.Ext(expectedBase))
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			nameWithoutExt := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
			if strings.EqualFold(nameWithoutExt, expectedNameWithoutExt) {
				filtered = append(filtered, file)
			}
		}
	} else {
		filtered = files
	}

	for _, file := range filtered {
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
	libPath := s.config.GetConfig().LibraryPath
	if libPath == "" {
		return fmt.Errorf("library path is not configured")
	}

	game, err := s.romm.GetRom(id)
	if err != nil {
		return fmt.Errorf("failed to get ROM info for deletion: %w", err)
	}

	romDir := s.GetRomDir(&game)
	if _, err := os.Stat(romDir); err == nil {
		if s.config.GetConfig().UsePlatformFolder {
			// In flat platform folder format, we must ONLY delete the files belonging to this game,
			// not the whole platform folder!
			expectedBase := filepath.Base(game.FullPath)
			expectedNameWithoutExt := strings.TrimSuffix(expectedBase, filepath.Ext(expectedBase))
			
			// 1. Delete ROM files in the platform directory
			files, err := os.ReadDir(romDir)
			if err == nil {
				for _, file := range files {
					if file.IsDir() {
						continue
					}
					nameWithoutExt := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
					if strings.EqualFold(nameWithoutExt, expectedNameWithoutExt) {
						_ = os.Remove(filepath.Join(romDir, file.Name()))
					}
				}
			}

			// 2. Delete metadata file
			metadataPath := s.GetMetadataPath(&game)
			_ = os.Remove(metadataPath)

			// 3. Delete saves and states
			for _, subDir := range []string{constants.DirSaves, constants.DirStates} {
				subDirPath := filepath.Join(romDir, subDir)
				_ = filepath.Walk(subDirPath, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return nil
					}
					nameWithoutExt := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
					if strings.EqualFold(nameWithoutExt, expectedNameWithoutExt) {
						_ = os.RemoveAll(path)
						if info.IsDir() {
							return filepath.SkipDir
						}
					}
					return nil
				})
			}
		} else {
			if err := os.RemoveAll(romDir); err != nil {
				s.ui.LogErrorf("DeleteRom: Error during RemoveAll for ID %d: %v", id, err)
				return fmt.Errorf("failed to delete ROM directory: %w", err)
			}
		}
		s.ui.LogInfof("DeleteRom: Successfully deleted ROM %d from library", id)
	}

	return nil
}

// FindRomPath is a public wrapper for finding a ROM path.
func (s *Service) FindRomPath(romDir string, game *types.Game) string {
	return s.findRomPath(romDir, game)
}

func (s *Service) GetBiosDir() string {
	return filepath.Join(s.config.GetConfig().LibraryPath, constants.DirBios)
}

// MigrateLibrary moves installed ROMs between ID folder layout and flat platform folder layout.
func (s *Service) MigrateLibrary(usePlatformFolder bool) error {
	libPath := s.config.GetConfig().LibraryPath
	if libPath == "" {
		return fmt.Errorf("library path is not configured")
	}

	// We temporarily retrieve all local games using the walk-based scanning.
	games, _, err := s.GetLocalLibrary(10000, 0, 0, "")
	if err != nil {
		return fmt.Errorf("failed to scan local library for migration: %w", err)
	}

	s.ui.LogInfof("MigrateLibrary: Found %d games to migrate. Target UsePlatformFolder: %t", len(games), usePlatformFolder)

	for _, game := range games {
		relPath := utils.SanitizePath(filepath.Dir(game.FullPath))
		platformDir := filepath.Join(libPath, relPath)
		idDir := filepath.Join(platformDir, fmt.Sprintf("%d", game.ID))

		expectedBase := filepath.Base(game.FullPath)
		expectedNameWithoutExt := strings.TrimSuffix(expectedBase, filepath.Ext(expectedBase))

		if usePlatformFolder {
			// Migrate from ID folder (idDir) to platform folder (platformDir)
			if info, err := os.Stat(idDir); err == nil && info.IsDir() {
				s.ui.LogInfof("MigrateLibrary: Migrating game %d (%s) to platform folder...", game.ID, game.Title)

				// 1. Move metadata: idDir/metadata.json -> platformDir/metadata_ID.json
				oldMeta := filepath.Join(idDir, "metadata.json")
				newMeta := filepath.Join(platformDir, fmt.Sprintf("metadata_%d.json", game.ID))
				if _, err := os.Stat(oldMeta); err == nil {
					_ = os.MkdirAll(platformDir, 0o755)
					_ = os.Rename(oldMeta, newMeta)
				}

				// 2. Move all other files in idDir to platformDir
				if err := moveDirectoryContents(idDir, platformDir); err != nil {
					s.ui.LogErrorf("MigrateLibrary: Failed to move directory contents for game %d: %v", game.ID, err)
				}

				// 3. Remove the empty ID directory
				_ = os.Remove(idDir)
			}
		} else {
			// Migrate from platform folder (platformDir) to ID folder (idDir)
			oldMeta := filepath.Join(platformDir, fmt.Sprintf("metadata_%d.json", game.ID))
			if _, err := os.Stat(oldMeta); err == nil {
				s.ui.LogInfof("MigrateLibrary: Migrating game %d (%s) to ID folder...", game.ID, game.Title)

				if err := os.MkdirAll(idDir, 0o755); err != nil {
					s.ui.LogErrorf("MigrateLibrary: Failed to create ID directory %s: %v", idDir, err)
					continue
				}

				// 1. Move metadata: platformDir/metadata_ID.json -> idDir/metadata.json
				newMeta := filepath.Join(idDir, "metadata.json")
				_ = os.Rename(oldMeta, newMeta)

				// 2. Move ROM files: find files in platformDir matching the ROM name without extension
				entries, err := os.ReadDir(platformDir)
				if err == nil {
					for _, entry := range entries {
						if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
							continue
						}
						nameWithoutExt := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
						if strings.EqualFold(nameWithoutExt, expectedNameWithoutExt) {
							srcFile := filepath.Join(platformDir, entry.Name())
							destFile := filepath.Join(idDir, entry.Name())
							if err := os.Rename(srcFile, destFile); err != nil {
								if err := copyFile(srcFile, destFile); err == nil {
									_ = os.Remove(srcFile)
								}
							}
						}
					}
				}

				// 3. Move saves and states from platformDir to idDir
				migrateSavesAndStates(platformDir, idDir, expectedNameWithoutExt)
			}
		}
	}

	return nil
}

func moveDirectoryContents(src, dest string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(destPath, 0o755); err != nil {
				return err
			}
			if err := moveDirectoryContents(srcPath, destPath); err != nil {
				return err
			}
			_ = os.Remove(srcPath)
		} else {
			if err := os.Rename(srcPath, destPath); err != nil {
				if err := copyFile(srcPath, destPath); err != nil {
					return err
				}
				_ = os.Remove(srcPath)
			}
		}
	}
	return nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func copyFileOrDir(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyFileOrDir(filepath.Join(src, entry.Name()), filepath.Join(dest, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	return copyFile(src, dest)
}

func migrateSavesAndStates(srcBase, destBase, expectedNameWithoutExt string) {
	for _, subDir := range []string{constants.DirSaves, constants.DirStates} {
		srcDir := filepath.Join(srcBase, subDir)
		if info, err := os.Stat(srcDir); err != nil || !info.IsDir() {
			continue
		}

		_ = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			nameWithoutExt := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
			if strings.EqualFold(nameWithoutExt, expectedNameWithoutExt) {
				rel, err := filepath.Rel(srcBase, path)
				if err == nil {
					destPath := filepath.Join(destBase, rel)
					_ = os.MkdirAll(filepath.Dir(destPath), 0o755)
					if err := os.Rename(path, destPath); err != nil {
						if err := copyFileOrDir(path, destPath); err == nil {
							_ = os.RemoveAll(path)
						}
					}
				}
				if info.IsDir() {
					return filepath.SkipDir
				}
			}
			return nil
		})
	}
}

// CleanupOrphanedRoms scans the library directory and deletes files/directories not tracked in metadata.
func (s *Service) CleanupOrphanedRoms() (int, error) {
	files, err := s.ScanOrphanedRoms()
	if err != nil {
		return 0, err
	}
	return s.DeleteOrphanedRoms(files)
}

// ScanOrphanedRoms scans the library directory and returns paths of files not tracked in metadata.
func (s *Service) ScanOrphanedRoms() ([]string, error) {
	libPath := s.config.GetConfig().LibraryPath
	if libPath == "" {
		return nil, fmt.Errorf("library path is not configured")
	}

	games, _, err := s.GetLocalLibrary(10000, 0, 0, "")
	if err != nil {
		return nil, fmt.Errorf("failed to list local library: %w", err)
	}

	trackedPaths := make(map[string]bool)
	biosDir := filepath.Clean(s.GetBiosDir())

	for _, game := range games {
		romDir := filepath.Clean(s.GetRomDir(&game))
		metaPath := filepath.Clean(s.GetMetadataPath(&game))
		trackedPaths[metaPath] = true

		romPath := s.findRomPath(romDir, &game)
		if romPath != "" {
			trackedPaths[filepath.Clean(romPath)] = true
			
			// For CUE/BIN games, also track the associated BIN files
			if strings.ToLower(filepath.Ext(romPath)) == ".cue" {
				expectedBase := filepath.Base(game.FullPath)
				expectedNameWithoutExt := strings.TrimSuffix(expectedBase, filepath.Ext(expectedBase))
				files, err := os.ReadDir(romDir)
				if err == nil {
					for _, file := range files {
						if file.IsDir() {
							continue
						}
						nameWithoutExt := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
						if strings.EqualFold(nameWithoutExt, expectedNameWithoutExt) {
							trackedPaths[filepath.Clean(filepath.Join(romDir, file.Name()))] = true
						}
					}
				}
			}
		}

		for _, subDir := range []string{constants.DirSaves, constants.DirStates} {
			subDirPath := filepath.Join(romDir, subDir)
			expectedBase := filepath.Base(game.FullPath)
			expectedNameWithoutExt := strings.TrimSuffix(expectedBase, filepath.Ext(expectedBase))
			_ = filepath.Walk(subDirPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				nameWithoutExt := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
				if strings.EqualFold(nameWithoutExt, expectedNameWithoutExt) {
					trackedPaths[filepath.Clean(path)] = true
				}
				return nil
			})
		}
	}

	var orphanedFiles []string

	err = filepath.Walk(libPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		cleanPath := filepath.Clean(path)
		if cleanPath == filepath.Clean(libPath) {
			return nil
		}

		if strings.HasPrefix(cleanPath, biosDir) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if !trackedPaths[cleanPath] {
			if strings.HasPrefix(info.Name(), ".") {
				return nil
			}
			rel, err := filepath.Rel(libPath, cleanPath)
			if err == nil {
				orphanedFiles = append(orphanedFiles, rel)
			} else {
				orphanedFiles = append(orphanedFiles, cleanPath)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return orphanedFiles, nil
}

// DeleteOrphanedRoms deletes the specified orphaned files and cleans up any empty directories.
func (s *Service) DeleteOrphanedRoms(files []string) (int, error) {
	libPath := s.config.GetConfig().LibraryPath
	if libPath == "" {
		return 0, fmt.Errorf("library path is not configured")
	}

	deletedCount := 0
	var dirsToCheck []string

	for _, file := range files {
		var fullPath string
		if filepath.IsAbs(file) {
			fullPath = filepath.Clean(file)
		} else {
			fullPath = filepath.Clean(filepath.Join(libPath, file))
		}

		if !strings.HasPrefix(fullPath, filepath.Clean(libPath)) || strings.HasPrefix(fullPath, filepath.Clean(s.GetBiosDir())) {
			s.ui.LogErrorf("DeleteOrphanedRoms: Blocked deletion of unsafe/bios path: %s", fullPath)
			continue
		}

		s.ui.LogInfof("DeleteOrphanedRoms: Deleting orphaned file: %s", fullPath)
		if err := os.Remove(fullPath); err == nil {
			deletedCount++
			parent := filepath.Dir(fullPath)
			for parent != filepath.Clean(libPath) && len(parent) > len(libPath) {
				dirsToCheck = append(dirsToCheck, parent)
				parent = filepath.Dir(parent)
			}
		} else if !os.IsNotExist(err) {
			s.ui.LogErrorf("DeleteOrphanedRoms: Failed to delete file %s: %v", fullPath, err)
		}
	}

	var allDirs []string
	_ = filepath.Walk(libPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		cleanPath := filepath.Clean(path)
		if cleanPath == filepath.Clean(libPath) || strings.HasPrefix(cleanPath, filepath.Clean(s.GetBiosDir())) {
			return nil
		}
		allDirs = append(allDirs, cleanPath)
		return nil
	})

	for i := 0; i < len(allDirs); i++ {
		for j := i + 1; j < len(allDirs); j++ {
			if len(allDirs[j]) > len(allDirs[i]) {
				allDirs[i], allDirs[j] = allDirs[j], allDirs[i]
			}
		}
	}

	for _, dir := range allDirs {
		entries, err := os.ReadDir(dir)
		if err == nil {
			if len(entries) == 1 && entries[0].Name() == ".DS_Store" {
				_ = os.Remove(filepath.Join(dir, ".DS_Store"))
				entries = nil
			}
			if len(entries) == 0 {
				s.ui.LogInfof("DeleteOrphanedRoms: Deleting empty directory: %s", dir)
				_ = os.Remove(dir)
			}
		}
	}

	return deletedCount, nil
}
