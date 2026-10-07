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
	"go-romm-sync/retroarch"
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

	if subDir == constants.DirSaves {
		_ = s.BridgeGameSaves(id, "")
	}

	dirPath := filepath.Join(s.library.GetRomDir(&game), subDir)

	var entries []os.DirEntry
	if e, err := os.ReadDir(dirPath); err == nil {
		entries = e
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	items = s.collectCoreFiles(&game, getPlatformSlug(&game), subDir, dirPath, entries)

	if subDir == constants.DirSaves && getPlatformSlug(&game) == "ps2" {
		pcsx2Dir := filepath.Join(s.library.GetBiosDir(), "pcsx2", "memcards")
		if pcsx2Items := s.scanFlatCoreFiles(&game, corePCSX2, pcsx2Dir); len(pcsx2Items) > 0 {
			items = append(items, pcsx2Items...)
		}
	}

	if subDir == constants.DirSaves {
		items = deduplicateSaveItems(items)
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

func (s *Service) collectCoreFiles(game *types.Game, platformSlug, subDir, dirPath string, entries []os.DirEntry) []types.FileItem {
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
				items = append(items, s.scanCoreDir(game, platformSlug, subDir, dirPath, entry.Name())...)
			}
		}
	}
	if subDir == constants.DirSaves {
		items = append(items, s.scanFlatCoreFiles(game, "", dirPath)...)
	}
	return items
}

func (s *Service) scanCoreDir(game *types.Game, platformSlug, subDir, dirPath, coreName string) []types.FileItem {
	coreDir := filepath.Join(dirPath, coreName)
	if coreName == coreDolphin {
		return s.scanDolphinFiles(platformSlug, coreDir)
	}
	if platformSlug == platformPSP && (coreName == corePPSSPP || coreName == corePPSSPP_LR) {
		return s.scanPPSSPPFiles(game, subDir, coreName, coreDir)
	}
	return s.scanFlatCoreFiles(game, coreName, coreDir)
}

func (s *Service) scanPPSSPPFiles(game *types.Game, subDir, coreName, coreDir string) []types.FileItem {
	if subDir != constants.DirSaves {
		return s.scanFlatCoreFiles(game, coreName, coreDir)
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

	if platformSlug == platformWii || strings.Contains(strings.ToLower(platformSlug), "wii") {
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
			items = append(items, s.scanFlatCoreFiles(nil, relCore, cardDir)...)
		}
	}

	return items
}

func (s *Service) scanFlatCoreFiles(game *types.Game, coreName, coreDir string) []types.FileItem {
	files, err := os.ReadDir(coreDir)
	if err != nil {
		return nil
	}

	var expectedNameWithoutExt string
	if game != nil {
		expectedBase := filepath.Base(game.FullPath)
		expectedNameWithoutExt = strings.TrimSuffix(expectedBase, filepath.Ext(expectedBase))
	}

	items := make([]types.FileItem, 0, len(files))
	for _, f := range files {
		if f.IsDir() || strings.HasPrefix(f.Name(), ".") || strings.HasSuffix(strings.ToLower(f.Name()), ".bak") || isRomFile(f.Name()) {
			continue
		}

		if game != nil && expectedNameWithoutExt != "" {
			if !strings.HasPrefix(strings.ToLower(f.Name()), strings.ToLower(expectedNameWithoutExt)) {
				continue
			}
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
func (s *Service) UploadSave(id uint, core, filename, slot string) error {
	return s.uploadServerAsset(id, core, filename, constants.DirSaves, slot)
}

// UploadState reads a local save state file and uploads it to RomM.
func (s *Service) UploadState(id uint, core, filename string) error {
	return s.uploadServerAsset(id, core, filename, constants.DirStates, "")
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

func (s *Service) uploadServerAsset(id uint, core, filename, subDir, slot string) error {
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
		err = s.romm.GetClient().UploadSave(id, core, filename, content, slot)
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

	if subDir == constants.DirSaves && !isSpecialSaveCore(core) {
		s.deleteAllCoreSaves(filepath.Join(romDir, constants.DirSaves), filename)
	}

	return nil
}

// DownloadServerSave downloads a save from RomM.
func (s *Service) DownloadServerSave(gameID, serverID uint, core, filename, updatedAt string) error {
	if err := s.downloadServerAsset(gameID, serverID, core, filename, updatedAt, constants.DirSaves); err != nil {
		return err
	}
	_ = s.BridgeGameSaves(gameID, "")
	return nil
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

	backupExistingFile(destPath)
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

func (s *Service) deleteAllCoreSaves(savesDir, filename string) {
	entries, err := os.ReadDir(savesDir)
	if err != nil {
		return
	}
	_ = os.Remove(filepath.Join(savesDir, filename))
	_ = os.Remove(filepath.Join(savesDir, filename+".bak"))
	for _, entry := range entries {
		if entry.IsDir() && !isSpecialSaveCore(entry.Name()) {
			_ = os.Remove(filepath.Join(savesDir, entry.Name(), filename))
			_ = os.Remove(filepath.Join(savesDir, entry.Name(), filename+".bak"))
		}
	}
}

type saveFileInfo struct {
	name    string
	core    string
	path    string
	modTime time.Time
}

var knownSaveExtensions = map[string]bool{
	".srm": true,
	".sav": true,
	".rtc": true,
	".eep": true,
	".fla": true,
	".dsv": true,
	".mpk": true,
	".sra": true,
	".ram": true,
	".raw": true,
	".ps2": true,
	".mcd": true,
	".mcr": true,
}

func isBatterySaveFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return knownSaveExtensions[ext]
}

var knownRomExtensions = map[string]bool{
	".zip":  true,
	".7z":   true,
	".rar":  true,
	".gba":  true,
	".gb":   true,
	".gbc":  true,
	".nes":  true,
	".sfc":  true,
	".smc":  true,
	".n64":  true,
	".z64":  true,
	".v64":  true,
	".nds":  true,
	".iso":  true,
	".bin":  true,
	".cue":  true,
	".chd":  true,
	".cso":  true,
	".pbp":  true,
	".wbfs": true,
	".gcm":  true,
	".rvz":  true,
	".m3u":  true,
	".elf":  true,
	".3ds":  true,
	".cia":  true,
}

func isRomFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return knownRomExtensions[ext]
}

func isSpecialSaveCore(core string) bool {
	if core == "" {
		return false
	}
	base := filepath.Base(strings.ReplaceAll(core, "\\", "/"))
	switch base {
	case corePCSX2, coreDolphin, corePPSSPP, corePPSSPP_LR, constants.CoreAzahar:
		return true
	}
	return strings.Contains(core, "dolphin") || strings.Contains(core, "pcsx2")
}

func backupExistingFile(path string) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return
	}
	bakPath := path + ".bak"
	_ = copyFileRawPreserveTime(path, bakPath, info.ModTime())
}

func copyFileRawPreserveTime(src, dst string, modTime time.Time) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close() //nolint:errcheck

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close() //nolint:errcheck

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	if err := dstFile.Close(); err != nil {
		return err
	}

	return os.Chtimes(dst, modTime, modTime)
}

func copyFilePreserveTime(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if dstInfo, err := os.Stat(dst); err == nil && !os.SameFile(srcInfo, dstInfo) {
		backupExistingFile(dst)
	}
	return copyFileRawPreserveTime(src, dst, srcInfo.ModTime())
}

func getGameExpectedSavePrefix(game *types.Game) string {
	if game == nil {
		return ""
	}
	base := filepath.Base(game.FullPath)
	if base == "." || base == "" {
		base = filepath.Base(game.FSName)
	}
	if base == "." || base == "" {
		base = game.Title
	}
	return strings.ToLower(strings.TrimSuffix(base, filepath.Ext(base)))
}

func scanCoreSaveFiles(savesDir, coreName, expectedPrefix string, newest map[string]saveFileInfo) {
	coreDir := filepath.Join(savesDir, coreName)
	files, err := os.ReadDir(coreDir)
	if err != nil {
		return
	}
	for _, f := range files {
		if f.IsDir() || strings.HasPrefix(f.Name(), ".") || !isBatterySaveFile(f.Name()) {
			continue
		}
		if expectedPrefix != "" && !strings.HasPrefix(strings.ToLower(f.Name()), expectedPrefix) {
			continue
		}
		info, err := f.Info()
		if err != nil {
			continue
		}
		existing, ok := newest[f.Name()]
		if !ok || info.ModTime().After(existing.modTime) {
			newest[f.Name()] = saveFileInfo{
				name:    f.Name(),
				core:    coreName,
				path:    filepath.Join(coreDir, f.Name()),
				modTime: info.ModTime(),
			}
		}
	}
}

func scanFlatSaveFiles(savesDir, expectedPrefix string, newest map[string]saveFileInfo) {
	files, err := os.ReadDir(savesDir)
	if err != nil {
		return
	}
	for _, f := range files {
		if f.IsDir() || strings.HasPrefix(f.Name(), ".") || !isBatterySaveFile(f.Name()) {
			continue
		}
		if expectedPrefix != "" && !strings.HasPrefix(strings.ToLower(f.Name()), expectedPrefix) {
			continue
		}
		info, err := f.Info()
		if err != nil {
			continue
		}
		existing, ok := newest[f.Name()]
		if !ok || info.ModTime().After(existing.modTime) {
			newest[f.Name()] = saveFileInfo{
				name:    f.Name(),
				core:    "",
				path:    filepath.Join(savesDir, f.Name()),
				modTime: info.ModTime(),
			}
		}
	}
}

func findNewestSaves(savesDir, expectedPrefix string, entries []os.DirEntry) map[string]saveFileInfo {
	newest := make(map[string]saveFileInfo)
	for _, entry := range entries {
		if entry.IsDir() && !isSpecialSaveCore(entry.Name()) {
			scanCoreSaveFiles(savesDir, entry.Name(), expectedPrefix, newest)
		}
	}
	scanFlatSaveFiles(savesDir, expectedPrefix, newest)
	return newest
}

func bridgeToTargetCore(savesDir, targetCore string, newest map[string]saveFileInfo) error {
	for saveName, saveInfo := range newest {
		if saveInfo.core == targetCore {
			continue
		}
		destPath := filepath.Join(savesDir, targetCore, saveName)
		if destInfo, err := os.Stat(destPath); err == nil && !destInfo.ModTime().Before(saveInfo.modTime) {
			continue
		}
		if err := copyFilePreserveTime(saveInfo.path, destPath); err != nil {
			return err
		}
	}
	return nil
}

func collectTargetCores(platformCores []string, entries []os.DirEntry) []string {
	seen := make(map[string]bool)
	var targets []string

	for _, entry := range entries {
		if entry.IsDir() && !isSpecialSaveCore(entry.Name()) {
			seen[entry.Name()] = true
			targets = append(targets, entry.Name())
		}
	}
	for _, core := range platformCores {
		if core != "" && !isSpecialSaveCore(core) && !seen[core] {
			seen[core] = true
			targets = append(targets, core)
		}
	}
	return targets
}

func bridgeAcrossAllCores(savesDir string, newest map[string]saveFileInfo, targetCores []string) error {
	for _, coreName := range targetCores {
		if err := bridgeToTargetCore(savesDir, coreName, newest); err != nil {
			return err
		}
	}
	for saveName, saveInfo := range newest {
		flatPath := filepath.Join(savesDir, saveName)
		if saveInfo.path == flatPath {
			continue
		}
		if destInfo, err := os.Stat(flatPath); err == nil && !destInfo.ModTime().Before(saveInfo.modTime) {
			continue
		}
		if err := copyFilePreserveTime(saveInfo.path, flatPath); err != nil {
			return err
		}
	}
	return nil
}

// BridgeGameSaves copies the newest battery saves across RetroArch cores for a game.
// If targetCore is specified, saves are bridged to that target core directory.
// If targetCore is empty, saves are bridged across all existing and platform core directories.
func (s *Service) BridgeGameSaves(id uint, targetCore string) error {
	if isSpecialSaveCore(targetCore) {
		return nil
	}
	game, err := s.library.GetLocalGame(id)
	if err != nil {
		game, err = s.romm.GetRom(id)
		if err != nil {
			return err
		}
	}
	platformSlug := getPlatformSlug(&game)
	if platformSlug == "ps2" || platformSlug == platformPSP {
		return nil
	}

	romDir := s.library.GetRomDir(&game)
	savesDir := filepath.Join(romDir, constants.DirSaves)
	_ = os.MkdirAll(savesDir, 0o755)

	entries, _ := os.ReadDir(savesDir)
	expectedPrefix := getGameExpectedSavePrefix(&game)
	newest := findNewestSaves(savesDir, expectedPrefix, entries)
	if len(newest) == 0 {
		return nil
	}

	if targetCore != "" {
		return bridgeToTargetCore(savesDir, targetCore, newest)
	}

	platformCores := retroarch.GetCoresForPlatform(platformSlug)
	targetCores := collectTargetCores(platformCores, entries)
	return bridgeAcrossAllCores(savesDir, newest, targetCores)
}

func deduplicateSaveItems(items []types.FileItem) []types.FileItem {
	itemMap := make(map[string]types.FileItem)
	order := make([]string, 0, len(items))

	for _, item := range items {
		existing, ok := itemMap[item.Name]
		if !ok {
			itemMap[item.Name] = item
			order = append(order, item.Name)
			continue
		}
		if item.UpdatedAt > existing.UpdatedAt || (item.UpdatedAt == existing.UpdatedAt && existing.Core == "" && item.Core != "") {
			itemMap[item.Name] = item
		}
	}

	result := make([]types.FileItem, 0, len(order))
	for _, name := range order {
		result = append(result, itemMap[name])
	}
	return result
}
