package sync

import (
	"context"
	"fmt"
	"go-romm-sync/library"
	"go-romm-sync/rommsrv"
	"go-romm-sync/types"
	"go-romm-sync/utils"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-romm-sync/constants"
	"go-romm-sync/utils/archive"
)

const (
	corePCSX2     = "pcsx2_libretro"
	azaharDirName = "Azahar"
	coreDolphin   = "dolphin-emu"
	wiiDirName    = "Wii"
	platformWii   = "wii"
	corePPSSPP    = "PPSSPP"
	corePPSSPP_LR = "ppsspp_libretro"
	platformPSP   = "psp"
)

// Service manages the synchronization of saves and states.
type Service struct {
	library *library.Service
	romm    *rommsrv.Service
	ui      types.UIProvider
}

// New creates a new Sync service.
func New(lib *library.Service, romm *rommsrv.Service, ui types.UIProvider) *Service {
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
	// Try local library first for metadata
	game, err := s.library.GetLocalGame(id)
	if err != nil {
		// Fallback to RomM if local metadata is missing
		game, err = s.romm.GetRom(id)
		if err != nil {
			return nil, err
		}
	}

	dirPath := filepath.Join(s.library.GetRomDir(&game), subDir)

	var entries []os.DirEntry
	if e, err := os.ReadDir(dirPath); err == nil {
		entries = e
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	items = s.collectCoreFiles(getPlatformSlug(&game), subDir, dirPath, entries)

	if subDir == constants.DirSaves && getPlatformSlug(&game) == "ps2" {
		pcsx2Dir := filepath.Join(s.library.GetBiosDir(), "pcsx2", "memcards")
		if pcsx2Items := s.scanFlatCoreFiles(corePCSX2, pcsx2Dir); len(pcsx2Items) > 0 {
			items = append(items, pcsx2Items...)
		}
	}

	return items, nil
}

func getPlatformSlug(game *types.Game) string {
	if game.Platform.Slug != "" {
		return game.Platform.Slug
	}
	if game.PlatformSlug != "" {
		return game.PlatformSlug
	}
	return ""
}

// (Deprecated/Removed handleGetFilesError logically)

func (s *Service) collectCoreFiles(platformSlug, subDir, dirPath string, entries []os.DirEntry) []types.FileItem {
	var items []types.FileItem
	for _, entry := range entries {
		if entry.IsDir() {
			if entry.Name() == azaharDirName {
				latestTime := getDirLatestModTime(filepath.Join(dirPath, entry.Name()))
				updatedAt := latestTime.UTC().Format(time.RFC3339)
				items = append(items, types.FileItem{
					Name:      entry.Name(),
					Core:      constants.CoreAzahar,
					UpdatedAt: updatedAt,
				})
			} else {
				items = append(items, s.scanCoreDir(platformSlug, subDir, dirPath, entry.Name())...)
			}
		}
	}
	return items
}

func (s *Service) scanCoreDir(platformSlug, subDir, dirPath, coreName string) []types.FileItem {
	coreDir := filepath.Join(dirPath, coreName)
	if coreName == coreDolphin {
		return s.scanDolphinFiles(platformSlug, coreDir)
	}
	if platformSlug == platformPSP && (coreName == corePPSSPP || coreName == corePPSSPP_LR) {
		return s.scanPPSSPPFiles(subDir, coreName, coreDir)
	}
	return s.scanFlatCoreFiles(coreName, coreDir)
}

func (s *Service) scanPPSSPPFiles(subDir, coreName, coreDir string) []types.FileItem {
	if subDir != constants.DirSaves {
		return s.scanFlatCoreFiles(coreName, coreDir)
	}

	saveDataDir := filepath.Join(coreDir, "PSP", "SAVEDATA")
	files, err := os.ReadDir(saveDataDir)
	if err != nil {
		return nil
	}

	items := make([]types.FileItem, 0, len(files))
	for _, f := range files {
		if !f.IsDir() || strings.HasPrefix(f.Name(), ".") {
			continue
		}
		items = append(items, types.FileItem{
			Name:      f.Name(),
			Core:      coreName,
			UpdatedAt: getDirLatestModTime(filepath.Join(saveDataDir, f.Name())).UTC().Format(time.RFC3339),
		})
	}
	return items
}

func (s *Service) scanDolphinFiles(platformSlug, coreDir string) []types.FileItem {
	items := make([]types.FileItem, 0, 4) // USA, EUR, JPN, Wii

	if platformSlug == platformWii {
		wiiDir := filepath.Join(coreDir, "User", wiiDirName)
		if info, err := os.Stat(wiiDir); err == nil && info.IsDir() {
			latestTime := getDirLatestModTime(wiiDir)
			updatedAt := latestTime.UTC().Format(time.RFC3339)
			items = append(items, types.FileItem{
				Name:      wiiDirName,
				Core:      coreDolphin,
				UpdatedAt: updatedAt,
			})
		}
	} else {
		gcDir := filepath.Join(coreDir, "User", "GC")
		for _, region := range []string{"USA", "EUR", "JPN"} {
			cardDir := filepath.Join(gcDir, region, "Card A")
			relCore := filepath.Join(coreDolphin, "User", "GC", region, "Card A")
			items = append(items, s.scanFlatCoreFiles(relCore, cardDir)...)
		}
	}

	return items
}

func (s *Service) scanFlatCoreFiles(coreName, coreDir string) []types.FileItem {
	files, err := os.ReadDir(coreDir)
	if err != nil {
		return nil
	}
	items := make([]types.FileItem, 0, len(files))
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

func getLocalAssetPaths(romDir, biosDir, subDir, core, filename, platform string) (baseDir, filePath string) {
	if core == corePCSX2 && subDir == constants.DirSaves {
		base := filepath.Join(biosDir, "pcsx2", "memcards")
		return base, filepath.Join(base, filename)
	}
	if platform == platformPSP && (core == corePPSSPP || core == corePPSSPP_LR) && subDir == constants.DirSaves {
		base := filepath.Join(romDir, subDir, core, "PSP", "SAVEDATA")
		return base, filepath.Join(base, filename)
	}
	base := filepath.Join(romDir, subDir)
	if core == constants.CoreAzahar && filename == azaharDirName {
		return base, filepath.Join(base, filename)
	}
	if core == coreDolphin && filename == wiiDirName {
		base = filepath.Join(base, coreDolphin, "User")
		return base, filepath.Join(base, wiiDirName)
	}
	return base, filepath.Join(base, core, filename)
}

func (s *Service) uploadServerAsset(id uint, core, filename, subDir string) error {
	game, err := s.library.GetLocalGame(id)
	if err != nil {
		game, err = s.romm.GetRom(id)
		if err != nil {
			return fmt.Errorf("failed to get ROM info: %w", err)
		}
	}

	romDir := s.library.GetRomDir(&game)
	baseDir, filePath := getLocalAssetPaths(romDir, s.library.GetBiosDir(), subDir, core, filename, getPlatformSlug(&game))

	cleanPath := filepath.Clean(filePath)
	cleanBase := filepath.Clean(baseDir)

	if !utils.IsSafePath(cleanBase, cleanPath) {
		return fmt.Errorf("invalid path traversal detected")
	}

	var content []byte
	info, statErr := os.Stat(cleanPath)
	if statErr == nil && info.IsDir() {
		var err error
		content, err = archive.ZipDirToBuffer(cleanPath)
		if err != nil {
			return fmt.Errorf("failed to zip directory %s: %w", cleanPath, err)
		}
	} else {
		content, err = os.ReadFile(cleanPath)
		if err != nil {
			return fmt.Errorf("failed to read local %s file: %w", subDir, err)
		}
	}

	if subDir == constants.DirSaves {
		err = s.romm.GetClient().UploadSave(id, core, filename, content)
	} else {
		err = s.romm.GetClient().UploadState(id, core, filename, content)
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
	game, err := s.library.GetLocalGame(id)
	if err != nil {
		game, err = s.romm.GetRom(id)
		if err != nil {
			return err
		}
	}

	romDir := s.library.GetRomDir(&game)
	baseDir, filePath := getLocalAssetPaths(romDir, s.library.GetBiosDir(), subDir, core, filename, getPlatformSlug(&game))

	cleanPath := filepath.Clean(filePath)
	cleanBase := filepath.Clean(baseDir)

	if !utils.IsSafePath(cleanBase, cleanPath) {
		return fmt.Errorf("invalid path traversal detected")
	}

	_, err = os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to access file %s: %w", cleanPath, err)
	}

	err = os.RemoveAll(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to delete file or directory %s: %w", cleanPath, err)
	}
	return nil
}

// DownloadServerSave downloads a save from RomM.
func (s *Service) DownloadServerSave(gameID, serverID uint, core, filename, updatedAt string) error {
	return s.downloadServerAsset(gameID, serverID, core, filename, updatedAt, constants.DirSaves)
}

// DownloadServerState downloads a state from RomM.
func (s *Service) DownloadServerState(gameID, serverID uint, core, filename, updatedAt string) error {
	return s.downloadServerAsset(gameID, serverID, core, filename, updatedAt, constants.DirStates)
}

func (s *Service) downloadServerAsset(gameID, serverID uint, core, filename, updatedAt, subDir string) error {
	game, err := s.library.GetLocalGame(gameID)
	if err != nil {
		game, err = s.romm.GetRom(gameID)
		if err != nil {
			return fmt.Errorf("failed to get ROM info: %w", err)
		}
	}

	ctx := context.Background()
	var reader io.ReadCloser
	var serverFilename string
	if subDir == constants.DirSaves {
		reader, serverFilename, err = s.romm.GetClient().DownloadSave(ctx, serverID)
	} else {
		reader, serverFilename, err = s.romm.GetClient().DownloadState(ctx, serverID)
	}

	if err != nil {
		return fmt.Errorf("failed to download %s from server: %w", subDir, err)
	}
	defer reader.Close() //nolint:errcheck

	if filename == "" {
		filename = serverFilename
	}

	destPath, err := s.prepareAssetPath(&game, core, filename, subDir)
	if err != nil {
		return err
	}

	if err := s.saveDownloadedAsset(reader, destPath, core, filename, subDir); err != nil {
		return err
	}

	if updatedAt != "" {
		s.setFileTime(destPath, updatedAt)
	}

	return nil
}

func (s *Service) saveDownloadedAsset(reader io.Reader, destPath, core, filename, subDir string) error {
	isDirAsset := (core == constants.CoreAzahar && filename == azaharDirName) ||
		(core == coreDolphin && filename == wiiDirName) ||
		((core == corePPSSPP || core == corePPSSPP_LR) && subDir == constants.DirSaves)

	if isDirAsset {
		tmpFile, err := os.CreateTemp("", "romm_dl_*.zip")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		defer func() {
			_ = tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
		}()
		if _, err := io.Copy(tmpFile, reader); err != nil {
			return fmt.Errorf("failed to download directory zip: %w", err)
		}
		if err := tmpFile.Close(); err != nil {
			return fmt.Errorf("failed to close temporary zip file: %w", err)
		}

		_ = os.RemoveAll(destPath)
		if _, err := archive.Extract(tmpFile.Name(), destPath); err != nil {
			return fmt.Errorf("failed to extract zip to %s: %w", destPath, err)
		}
		return nil
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create local %s file: %w", subDir, err)
	}
	defer out.Close() //nolint:errcheck

	if _, err := io.Copy(out, reader); err != nil {
		return fmt.Errorf("failed to write local %s file: %w", subDir, err)
	}
	return nil
}

func (s *Service) prepareAssetPath(game *types.Game, core, filename, subDir string) (string, error) {
	core, filename, err := s.ValidateAssetPath(core, filename)
	if err != nil {
		return "", err
	}

	if destDir := s.getSpecialDestDir(game, core, filename, subDir); destDir != "" {
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return "", fmt.Errorf("failed to create destination directory: %w", err)
		}
		return filepath.Join(destDir, filename), nil
	}

	baseDir := filepath.Join(s.library.GetRomDir(game), subDir)
	core = remapCorePath(core)
	destDir := filepath.Join(baseDir, core)

	if !utils.IsSafePath(baseDir, destDir) {
		return "", fmt.Errorf("invalid path traversal detected")
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create destination directory: %w", err)
	}

	return filepath.Join(destDir, filename), nil
}

func (s *Service) getSpecialDestDir(game *types.Game, core, filename, subDir string) string {
	if core == corePCSX2 && subDir == constants.DirSaves {
		return filepath.Join(s.library.GetBiosDir(), "pcsx2", "memcards")
	}
	if core == constants.CoreAzahar && filename == azaharDirName {
		return filepath.Join(s.library.GetRomDir(game), subDir)
	}
	if getPlatformSlug(game) == platformPSP && (core == corePPSSPP || core == corePPSSPP_LR) && subDir == constants.DirSaves {
		return filepath.Join(s.library.GetRomDir(game), subDir, core, "PSP", "SAVEDATA")
	}
	return ""
}

func remapCorePath(core string) string {
	// Remap the Dolphin "Card A" / "Card B" emulator names from RomM to the correct
	// local nested path that the dolphin-emu RetroArch core expects.
	// RomM stores these saves with emulator = "Card A", but locally they must live at:
	//   saves/dolphin-emu/User/GC/{region}/Card A/
	// We default to USA region; the file will be placed correctly for NTSC-U games.
	//
	// Normalize backslashes to forward slashes before extracting the base name so
	// that saves uploaded from Windows (where the emulator field was stored with
	// backslash separators, e.g. "dolphin-emu\User\GC\USA\Card A") are handled
	// correctly on macOS/Linux where filepath.Base does not treat '\' as a separator.
	coreBase := filepath.Base(strings.ReplaceAll(core, "\\", "/"))
	switch coreBase {
	case "Card A":
		return filepath.Join("dolphin-emu", "User", "GC", "USA", "Card A")
	case "Card B":
		return filepath.Join("dolphin-emu", "User", "GC", "USA", "Card B")
	}
	return core
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

func getDirLatestModTime(dirPath string) time.Time {
	var latest time.Time
	_ = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	// If no files found, fallback to directory mtime
	if latest.IsZero() {
		if info, err := os.Stat(dirPath); err == nil {
			return info.ModTime()
		}
	}
	return latest
}
