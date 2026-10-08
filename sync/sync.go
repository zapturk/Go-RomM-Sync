package sync

import (
	"bytes"
	"context"
	"fmt"
	"go-romm-sync/library"
	"go-romm-sync/romm"
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

var (
	dolphinGCRegions = []string{"USA", "EUR", "JAP", "JPN"}
	dolphinGCCards   = []string{"Card A", "Card B"}
)

func isGameCubePlatform(platform string) bool {
	slug := strings.ToLower(platform)
	switch slug {
	case "gamecube", "gc", "ngc", "gcn":
		return true
	}
	return retroarch.IdentifyPlatform(slug) == "gamecube"
}

func isWiiPlatform(platform string) bool {
	slug := strings.ToLower(platform)
	switch slug {
	case "wii", "wiiware":
		return true
	}
	return retroarch.IdentifyPlatform(slug) == "wii"
}

func isDolphinCore(core string) bool {
	norm := strings.ToLower(strings.ReplaceAll(core, "\\", "/"))
	return strings.Contains(norm, "dolphin")
}

func isGameCubeSave(platform, core, filename string) bool {
	if strings.EqualFold(filename, "sram.raw") {
		return false
	}
	if isGameCubePlatform(platform) {
		return true
	}
	normCore := strings.ToLower(strings.ReplaceAll(core, "\\", "/"))
	if strings.Contains(normCore, "user/gc") || normCore == "card a" || normCore == "card b" {
		return true
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".gci" || ext == ".gcp" {
		return true
	}
	upperFile := strings.ToUpper(filename)
	return (ext == ".raw" || ext == ".sav") && (strings.HasPrefix(upperFile, "MEMORYCARDA") || strings.HasPrefix(upperFile, "MEMORYCARDB"))
}

// Service manages the synchronization of saves and states.
type Service struct {
	library *library.Service
	romm    *rommsrv.Service
	ui      types.UIProvider
}

// New creates a new Sync service.
func New(lib *library.Service, rommSrv *rommsrv.Service, ui types.UIProvider) *Service {
	return &Service{
		library: lib,
		romm:    rommSrv,
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
		_ = s.FlattenGameSaves(id)
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
		if !entry.IsDir() {
			continue
		}
		switch {
		case entry.Name() == azaharDirName:
			latestTime := getDirLatestModTime(filepath.Join(dirPath, entry.Name()))
			updatedAt := latestTime.UTC().Format(time.RFC3339)
			items = append(items, types.FileItem{
				Name:      entry.Name(),
				Core:      constants.CoreAzahar,
				UpdatedAt: updatedAt,
			})
		case entry.Name() == "User" && (isGameCubePlatform(platformSlug) || isWiiPlatform(platformSlug)) && subDir == constants.DirSaves:
			dolphinDir := filepath.Join(dirPath, coreDolphin)
			items = append(items, s.scanDolphinFiles(platformSlug, dolphinDir)...)
		default:
			items = append(items, s.scanCoreDir(game, platformSlug, subDir, dirPath, entry.Name())...)
		}
	}
	if subDir == constants.DirSaves {
		defaultCore := s.getGameDefaultCore(game)
		items = append(items, s.scanFlatCoreFiles(game, defaultCore, dirPath)...)
	}
	return items
}

func (s *Service) scanCoreDir(game *types.Game, platformSlug, subDir, dirPath, coreName string) []types.FileItem {
	coreDir := filepath.Join(dirPath, coreName)
	if isDolphinCore(coreName) && subDir == constants.DirSaves {
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
	retroarch.MigrateDolphinUserDir(filepath.Dir(coreDir))
	items := make([]types.FileItem, 0, 8)

	if isWiiPlatform(platformSlug) {
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
		for _, region := range dolphinGCRegions {
			for _, card := range dolphinGCCards {
				cardDir := filepath.Join(gcDir, region, card)
				relCore := filepath.ToSlash(filepath.Join(coreDolphin, "User", "GC", region, card))
				items = append(items, s.scanFlatCoreFiles(nil, relCore, cardDir)...)
			}
		}

		if gcEntries, err := os.ReadDir(gcDir); err == nil {
			for _, e := range gcEntries {
				if e.IsDir() || strings.HasPrefix(e.Name(), ".") || strings.HasSuffix(strings.ToLower(e.Name()), ".bak") || isRomFile(e.Name()) || strings.EqualFold(e.Name(), "sram.raw") {
					continue
				}
				if isBatterySaveFile(e.Name()) {
					info, err := e.Info()
					updatedAt := ""
					if err == nil {
						updatedAt = info.ModTime().UTC().Format(time.RFC3339)
					}
					relCore := filepath.ToSlash(filepath.Join(coreDolphin, "User", "GC"))
					items = append(items, types.FileItem{
						Name:      e.Name(),
						Core:      relCore,
						UpdatedAt: updatedAt,
					})
				}
			}
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
	if isDolphinCore(core) && filename == wiiDirName {
		base = filepath.Join(base, coreDolphin, "User")
		return base, filepath.Join(base, wiiDirName)
	}
	if isGameCubeSave(platform, core, filename) && subDir == constants.DirSaves {
		return base, findOrResolveGameCubeSavePath(base, core, filename, platform)
	}
	if subDir == constants.DirSaves && !isSpecialSaveCore(core) {
		flatPath := filepath.Join(base, filename)
		if _, err := os.Stat(flatPath); err == nil {
			return base, flatPath
		}
		corePath := filepath.Join(base, core, filename)
		if _, err := os.Stat(corePath); err == nil {
			return base, corePath
		}
		return base, flatPath
	}
	return base, filepath.Join(base, core, filename)
}

func findOrResolveGameCubeSavePath(savesDir, core, filename, platform string) string {
	normCore := filepath.ToSlash(core)
	if strings.HasPrefix(normCore, coreDolphin+"/User/GC") {
		candidate := filepath.Join(savesDir, filepath.FromSlash(normCore), filename)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	gcBase := filepath.Join(savesDir, coreDolphin, "User", "GC")
	for _, reg := range dolphinGCRegions {
		for _, card := range dolphinGCCards {
			candidate := filepath.Join(gcBase, reg, card, filename)
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}

	directGC := filepath.Join(gcBase, filename)
	if _, err := os.Stat(directGC); err == nil {
		return directGC
	}

	directDolphin := filepath.Join(savesDir, coreDolphin, filename)
	if _, err := os.Stat(directDolphin); err == nil {
		return directDolphin
	}

	flatCandidate := filepath.Join(savesDir, filename)
	if _, err := os.Stat(flatCandidate); err == nil {
		return flatCandidate
	}

	if strings.HasPrefix(normCore, coreDolphin+"/User/GC") {
		return filepath.Join(savesDir, filepath.FromSlash(normCore), filename)
	}
	region := resolveGameCubeRegion(nil, core, filename, savesDir)
	card := resolveGameCubeCard(core, filename)
	return filepath.Join(gcBase, region, card, filename)
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
		filename = romm.CleanSaveFileName(serverFilename)
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
		(isDolphinCore(core) && filename == wiiDirName) ||
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
	if destDir := s.getSpecialDestDir(game, core, filename, subDir); destDir != "" {
		cleanedFilename := filepath.Base(filepath.Clean(filename))
		if cleanedFilename == "." || cleanedFilename == ".." {
			return "", fmt.Errorf("invalid filename")
		}
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return "", fmt.Errorf("failed to create destination directory: %w", err)
		}
		destPath := filepath.Join(destDir, cleanedFilename)
		romDir := s.library.GetRomDir(game)
		baseDir := filepath.Join(romDir, subDir)
		if destDir == filepath.Join(s.library.GetBiosDir(), "pcsx2", "memcards") {
			baseDir = s.library.GetBiosDir()
		}
		if !utils.IsSafePath(baseDir, destPath) {
			return "", fmt.Errorf("invalid path traversal detected")
		}
		return destPath, nil
	}

	core, filename, err := s.ValidateAssetPath(core, filename)
	if err != nil {
		return "", err
	}

	baseDir := filepath.Join(s.library.GetRomDir(game), subDir)
	if subDir == constants.DirSaves && !isSpecialSaveCore(core) {
		if err := os.MkdirAll(baseDir, 0o755); err != nil {
			return "", fmt.Errorf("failed to create destination directory: %w", err)
		}
		return filepath.Join(baseDir, filename), nil
	}

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
	if isGameCubeSave(getPlatformSlug(game), core, filename) && subDir == constants.DirSaves {
		return s.getGameCubeDestDir(game, core, filename)
	}
	if (isWiiPlatform(getPlatformSlug(game)) || isDolphinCore(core)) && filename == wiiDirName && subDir == constants.DirSaves {
		return filepath.Join(s.library.GetRomDir(game), subDir, coreDolphin, "User")
	}
	return ""
}

func (s *Service) getGameCubeDestDir(game *types.Game, core, filename string) string {
	romDir := s.library.GetRomDir(game)
	savesDir := filepath.Join(romDir, constants.DirSaves)
	gcBase := filepath.Join(savesDir, coreDolphin, "User", "GC")

	normCore := filepath.ToSlash(core)
	if strings.HasPrefix(normCore, coreDolphin+"/User/GC/") {
		return filepath.Join(savesDir, filepath.FromSlash(normCore))
	}
	if normCore == coreDolphin+"/User/GC" {
		return gcBase
	}

	upperFile := strings.ToUpper(filename)
	ext := strings.ToLower(filepath.Ext(filename))
	if (ext == ".raw" || ext == ".gcp") && (strings.HasPrefix(upperFile, "MEMORYCARDA") || strings.HasPrefix(upperFile, "MEMORYCARDB")) &&
		!strings.Contains(strings.ToUpper(core), "CARD") {
		return gcBase
	}

	region := resolveGameCubeRegion(game, core, filename, savesDir)
	card := resolveGameCubeCard(core, filename)
	return filepath.Join(gcBase, region, card)
}

func resolveGameCubeCard(core, filename string) string {
	upperCore := strings.ToUpper(strings.ReplaceAll(core, "\\", "/"))
	upperFile := strings.ToUpper(filename)
	if strings.Contains(upperCore, "CARD B") || strings.Contains(upperFile, "CARD B") || strings.HasPrefix(upperFile, "MEMORYCARDB") {
		return "Card B"
	}
	return "Card A"
}

func normalizeRegion(reg string) string {
	if reg == "JPN" {
		return "JAP"
	}
	return reg
}

func resolveGameCubeRegion(game *types.Game, core, filename, savesDir string) string {
	upperCore := strings.ToUpper(strings.ReplaceAll(core, "\\", "/"))
	for _, reg := range dolphinGCRegions {
		if strings.Contains(upperCore, "/"+reg+"/") || strings.HasSuffix(upperCore, "/"+reg) || upperCore == reg {
			return normalizeRegion(reg)
		}
	}

	upperFile := strings.ToUpper(filename)
	for _, reg := range dolphinGCRegions {
		if strings.Contains(upperFile, "."+reg+".") || strings.Contains(upperFile, "_"+reg+"_") || strings.Contains(upperFile, "("+reg+")") {
			return normalizeRegion(reg)
		}
	}

	if reg := detectRegionFromGCI(filename); reg != "" {
		return reg
	}

	gcBase := filepath.Join(savesDir, coreDolphin, "User", "GC")
	for _, reg := range dolphinGCRegions {
		if info, err := os.Stat(filepath.Join(gcBase, reg)); err == nil && info.IsDir() {
			return normalizeRegion(reg)
		}
	}

	if game != nil {
		combined := strings.ToUpper(game.Title + " " + game.FullPath)
		if strings.Contains(combined, "EUROPE") || strings.Contains(combined, "(EUR)") || strings.Contains(combined, "(PAL)") {
			return "EUR"
		}
		if strings.Contains(combined, "JAPAN") || strings.Contains(combined, "(JAP)") || strings.Contains(combined, "(JPN)") {
			return "JAP"
		}
	}

	return "USA"
}

func detectRegionFromGCI(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".gci" {
		return ""
	}
	base := strings.TrimSuffix(filepath.Base(filename), ext)
	for _, p := range strings.Split(base, "-") {
		if len(p) >= 4 && isGameID(p[:4]) {
			return regionCodeFromChar(p[3])
		}
	}
	return ""
}

func isGameID(s string) bool {
	if len(s) != 4 {
		return false
	}
	s = strings.ToUpper(s)
	for i := 0; i < 3; i++ {
		c := s[i]
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return regionCodeFromChar(s[3]) != ""
}

func regionCodeFromChar(c byte) string {
	switch c {
	case 'E', 'e':
		return "USA"
	case 'P', 'p', 'X', 'x', 'Y', 'y', 'D', 'd', 'F', 'f', 'I', 'i', 'S', 's', 'U', 'u':
		return "EUR"
	case 'J', 'j':
		return "JAP"
	default:
		return ""
	}
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
	".gci": true,
	".gcp": true,
}

func isBatterySaveFile(filename string) bool {
	if strings.EqualFold(filename, "sram.raw") {
		return false
	}
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
	case corePCSX2, coreDolphin, corePPSSPP, corePPSSPP_LR, constants.CoreAzahar, "Card A", "Card B", "User":
		return true
	}
	norm := strings.ToLower(strings.ReplaceAll(core, "\\", "/"))
	return strings.Contains(norm, "dolphin") || strings.Contains(norm, "pcsx2") || strings.Contains(norm, "user/gc")
}

func backupExistingFile(path string) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return
	}
	bakPath := path + ".bak"
	if bakInfo, err := os.Stat(bakPath); err == nil && !bakInfo.IsDir() {
		if sameFileContent(path, bakPath) {
			return
		}
		bakPath = path + "." + info.ModTime().UTC().Format("20060102150405") + ".bak"
	}
	_ = copyFileRawPreserveTime(path, bakPath, info.ModTime())
}

func sameFileContent(path1, path2 string) bool {
	f1, err := os.Open(path1)
	if err != nil {
		return false
	}
	defer f1.Close() //nolint:errcheck

	f2, err := os.Open(path2)
	if err != nil {
		return false
	}
	defer f2.Close() //nolint:errcheck

	s1, err := f1.Stat()
	if err != nil {
		return false
	}
	s2, err := f2.Stat()
	if err != nil {
		return false
	}
	if os.SameFile(s1, s2) {
		return true
	}
	if s1.Size() != s2.Size() {
		return false
	}

	buf1 := make([]byte, 32*1024)
	buf2 := make([]byte, 32*1024)
	for {
		n1, err1 := io.ReadFull(f1, buf1)
		n2, err2 := io.ReadFull(f2, buf2)
		if n1 != n2 || !bytes.Equal(buf1[:n1], buf2[:n2]) {
			return false
		}
		if err1 == io.EOF || err1 == io.ErrUnexpectedEOF {
			return err2 == io.EOF || err2 == io.ErrUnexpectedEOF
		}
		if err1 != nil || err2 != nil {
			return false
		}
	}
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

func (s *Service) getGameDefaultCore(game *types.Game) string {
	if game == nil {
		return ""
	}
	platformSlug := getPlatformSlug(game)
	cores := retroarch.GetCoresForPlatform(platformSlug)
	if len(cores) > 0 {
		return cores[0]
	}
	return ""
}

// FlattenGameSaves migrates any battery saves from legacy core subdirectories into the root saves directory.
// For any conflict, the newer save is preserved and the replaced save is backed up to .bak.
// Empty core subdirectories are cleaned up after migration.
func (s *Service) FlattenGameSaves(id uint) error {
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
	if isWiiPlatform(platformSlug) {
		romDir := s.library.GetRomDir(&game)
		retroarch.MigrateDolphinUserDir(filepath.Join(romDir, constants.DirSaves))
		return nil
	}
	if isGameCubePlatform(platformSlug) {
		s.migrateMisplacedGameCubeSaves(&game)
		return nil
	}

	romDir := s.library.GetRomDir(&game)
	savesDir := filepath.Join(romDir, constants.DirSaves)
	if _, err := os.Stat(savesDir); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(savesDir)
	if err != nil {
		return err
	}

	expectedPrefix := ""
	if s.library.UsesPlatformFolder() {
		expectedPrefix = getGameExpectedSavePrefix(&game)
	}

	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") && !isSpecialSaveCore(entry.Name()) {
			migrateCoreDirSaves(savesDir, filepath.Join(savesDir, entry.Name()), expectedPrefix)
		}
	}
	return nil
}

func (s *Service) migrateMisplacedGameCubeSaves(game *types.Game) {
	romDir := s.library.GetRomDir(game)
	savesDir := filepath.Join(romDir, constants.DirSaves)
	retroarch.MigrateDolphinUserDir(savesDir)
	entries, err := os.ReadDir(savesDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		upper := strings.ToUpper(e.Name())
		if ext == ".gci" || ext == ".gcp" || ((ext == ".raw" || ext == ".sav") && strings.HasPrefix(upper, "MEMORYCARD")) {
			src := filepath.Join(savesDir, e.Name())
			destDir := s.getGameCubeDestDir(game, "", e.Name())
			_ = os.MkdirAll(destDir, 0o755)
			dst := filepath.Join(destDir, e.Name())
			if _, err := os.Stat(dst); os.IsNotExist(err) {
				_ = os.Rename(src, dst)
			}
		}
	}
}

func migrateCoreDirSaves(savesDir, coreDir, expectedPrefix string) {
	coreFiles, err := os.ReadDir(coreDir)
	if err != nil {
		return
	}
	for _, cf := range coreFiles {
		if cf.IsDir() || strings.HasPrefix(cf.Name(), ".") {
			continue
		}
		if expectedPrefix != "" && !strings.HasPrefix(strings.ToLower(cf.Name()), expectedPrefix) {
			continue
		}
		if isBatterySaveFile(cf.Name()) {
			migrateCoreSaveFile(savesDir, coreDir, cf.Name())
		} else if strings.HasSuffix(strings.ToLower(cf.Name()), ".bak") {
			baseName := strings.TrimSuffix(cf.Name(), ".bak")
			if isBatterySaveFile(baseName) {
				migrateOrphanBackup(savesDir, coreDir, cf.Name(), baseName)
			}
		}
	}

	cleanEmptyCoreDir(coreDir)
}

func cleanEmptyCoreDir(coreDir string) {
	remaining, err := os.ReadDir(coreDir)
	if err != nil {
		return
	}
	for _, r := range remaining {
		if !strings.HasPrefix(r.Name(), ".") {
			return
		}
	}
	_ = os.RemoveAll(coreDir)
}

func migrateCoreSaveFile(savesDir, coreDir, fileName string) {
	srcPath := filepath.Join(coreDir, fileName)
	dstPath := filepath.Join(savesDir, fileName)
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return
	}

	if dstInfo, err := os.Stat(dstPath); err == nil {
		handleExistingSaveConflict(srcPath, dstPath, coreDir, srcInfo, dstInfo)
	} else {
		_ = copyFileRawPreserveTime(srcPath, dstPath, srcInfo.ModTime())
		_ = os.Remove(srcPath)
	}

	migrateSaveBackup(srcPath, dstPath, coreDir)
}

func handleExistingSaveConflict(srcPath, dstPath, coreDir string, srcInfo, dstInfo os.FileInfo) {
	if sameFileContent(srcPath, dstPath) {
		_ = os.Remove(srcPath)
		return
	}

	if srcInfo.ModTime().After(dstInfo.ModTime()) {
		backupExistingFile(dstPath)
		_ = copyFileRawPreserveTime(srcPath, dstPath, srcInfo.ModTime())
	} else {
		preserveOlderSave(srcPath, dstPath, coreDir, srcInfo.ModTime())
	}
	_ = os.Remove(srcPath)
}

func preserveOlderSave(srcPath, dstPath, coreDir string, modTime time.Time) {
	dstBak := dstPath + ".bak"
	if _, err := os.Stat(dstBak); os.IsNotExist(err) {
		_ = copyFileRawPreserveTime(srcPath, dstBak, modTime)
		return
	}
	if !sameFileContent(srcPath, dstBak) {
		coreBak := dstPath + "." + filepath.Base(coreDir) + ".bak"
		_ = copyFileRawPreserveTime(srcPath, coreBak, modTime)
	}
}

func migrateSaveBackup(srcPath, dstPath, coreDir string) {
	srcBak := srcPath + ".bak"
	srcBakInfo, err := os.Stat(srcBak)
	if err != nil {
		return
	}

	dstBak := dstPath + ".bak"
	if _, err := os.Stat(dstBak); os.IsNotExist(err) {
		_ = os.Rename(srcBak, dstBak)
		return
	}

	if sameFileContent(srcBak, dstBak) {
		_ = os.Remove(srcBak)
		return
	}

	coreBak := dstPath + "." + filepath.Base(coreDir) + ".bak"
	if _, err := os.Stat(coreBak); os.IsNotExist(err) {
		_ = os.Rename(srcBak, coreBak)
		return
	}

	if sameFileContent(srcBak, coreBak) {
		_ = os.Remove(srcBak)
		return
	}

	tsBak := dstPath + "." + srcBakInfo.ModTime().UTC().Format("20060102150405") + ".bak"
	_ = copyFileRawPreserveTime(srcBak, tsBak, srcBakInfo.ModTime())
	_ = os.Remove(srcBak)
}

func migrateOrphanBackup(savesDir, coreDir, fileName, baseName string) {
	srcBak := filepath.Join(coreDir, fileName)
	srcBakInfo, err := os.Stat(srcBak)
	if err != nil {
		return
	}

	dstBak := filepath.Join(savesDir, fileName)
	if _, err := os.Stat(dstBak); os.IsNotExist(err) {
		_ = os.Rename(srcBak, dstBak)
		return
	}

	if sameFileContent(srcBak, dstBak) {
		_ = os.Remove(srcBak)
		return
	}

	coreBak := filepath.Join(savesDir, baseName+"."+filepath.Base(coreDir)+".bak")
	if _, err := os.Stat(coreBak); os.IsNotExist(err) {
		_ = os.Rename(srcBak, coreBak)
		return
	}

	if sameFileContent(srcBak, coreBak) {
		_ = os.Remove(srcBak)
		return
	}

	tsBak := filepath.Join(savesDir, baseName+"."+srcBakInfo.ModTime().UTC().Format("20060102150405")+".bak")
	_ = copyFileRawPreserveTime(srcBak, tsBak, srcBakInfo.ModTime())
	_ = os.Remove(srcBak)
}

// BridgeGameSaves flattens and migrates battery saves into the root saves directory.
// Maintained for backward compatibility with callers expecting a bridge/sync call.
func (s *Service) BridgeGameSaves(id uint, targetCore string) error {
	return s.FlattenGameSaves(id)
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
