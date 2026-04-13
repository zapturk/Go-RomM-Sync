package firmware

import (
	"context"
	"fmt"
	"go-romm-sync/constants"
	"go-romm-sync/retroarch"
	"go-romm-sync/types"
	"go-romm-sync/utils/archive"
	"go-romm-sync/utils/fileio"
	"io"
	"os"
	"path/filepath"
)

const platformPS2 = "ps2"

// ConfigProvider defines the configuration needed for firmware management.
type ConfigProvider interface {
	GetLibraryPath() string
}

// RomMProvider defines the RomM API interactions needed for firmware management.
type RomMProvider interface {
	DownloadFirmwareContent(ctx context.Context, id uint, fileName string) (io.ReadCloser, string, error)
}

// UIProvider defines logging functionality.
type UIProvider interface {
	LogInfof(format string, args ...interface{})
	LogErrorf(format string, args ...interface{})
}

// Service manages BIOS and firmware files.
type Service struct {
	config ConfigProvider
	romm   RomMProvider
	ui     UIProvider
}

// New creates a new Firmware service.
func New(cfg ConfigProvider, romm RomMProvider, ui UIProvider) *Service {
	return &Service{
		config: cfg,
		romm:   romm,
		ui:     ui,
	}
}

// GetBiosDir returns the local directory where BIOS files are stored.
func (s *Service) GetBiosDir() string {
	return filepath.Join(s.config.GetLibraryPath(), constants.DirBios)
}

// IsFirmwareDownloaded checks if the firmware files are already locally available.
func (s *Service) IsFirmwareDownloaded(platformSlug string, fw *types.Firmware) bool {
	biosDir := s.GetBiosDir()
	subDir := ""
	if platformSlug == platformPS2 {
		subDir = filepath.Join("pcsx2", "bios")
	}
	targetDir := filepath.Join(biosDir, subDir)

	canonicalNames := retroarch.GetBiosFilenamesForPlatform(platformSlug)
	for _, name := range canonicalNames {
		if _, err := os.Stat(filepath.Join(targetDir, name)); err == nil {
			return true
		}
	}

	expectedName := fw.FileName
	if md5 := fw.MD5Hash; md5 != "" {
		if mapped := retroarch.GetBiosFilename(md5); mapped != "" {
			expectedName = mapped
		}
	}
	if expectedName != "" {
		if _, err := os.Stat(filepath.Join(targetDir, expectedName)); err == nil {
			return true
		}
	}

	return false
}

// DownloadFirmware downloads a firmware file to the library's bios directory.
func (s *Service) DownloadFirmware(platformSlug string, fw *types.Firmware) error {
	biosDir := s.GetBiosDir()
	if err := os.MkdirAll(biosDir, 0o755); err != nil {
		return fmt.Errorf("failed to create bios directory: %w", err)
	}

	ctx := context.Background()
	reader, filename, err := s.romm.DownloadFirmwareContent(ctx, fw.ID, fw.FileName)
	if err != nil {
		return err
	}
	defer fileio.Close(reader, s.ui.LogErrorf, "DownloadFirmware: Failed to close reader")

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
			return s.processBiosFile(path, biosDir, platformSlug, "", "")
		})
	}

	// Not an archive, process single file
	return s.processBiosFile(tempFile, biosDir, platformSlug, filename, fw.MD5Hash)
}

func (s *Service) processBiosFile(sourcePath, biosDir, platformSlug, origFilename, providedMD5 string) error {
	md5 := providedMD5
	if md5 == "" {
		var err error
		md5, err = fileio.GetMD5(sourcePath)
		if err != nil {
			s.ui.LogErrorf("processBiosFile: Failed to calculate MD5 for %s: %v", sourcePath, err)
		}
	}

	destFilename := filepath.Base(sourcePath)
	if origFilename != "" {
		destFilename = origFilename
	}

	if mappedName := retroarch.GetBiosFilename(md5); mappedName != "" {
		s.ui.LogInfof("processBiosFile: Mapping BIOS MD5 %s to canonical name %s (orig: %s)", md5, mappedName, destFilename)
		destFilename = mappedName
	}

	subDir := ""
	if platformSlug == platformPS2 {
		subDir = filepath.Join("pcsx2", "bios")
	}

	targetDir := filepath.Join(biosDir, subDir)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		s.ui.LogErrorf("processBiosFile: Failed to create target dir %s: %v", targetDir, err)
	}

	destPath := filepath.Join(targetDir, destFilename)
	// Move file to final destination
	return os.Rename(sourcePath, destPath)
}

// CleanupFirmware removes known canonical BIOS files from the bios directory for a specific platform.
func (s *Service) CleanupFirmware(platformSlug string) error {
	biosDir := s.GetBiosDir()
	biosNames := retroarch.GetBiosFilenamesForPlatform(platformSlug)

	for _, name := range biosNames {
		subDir := ""
		if platformSlug == platformPS2 {
			subDir = filepath.Join("pcsx2", "bios")
		}
		path := filepath.Join(biosDir, subDir, name)
		if _, err := os.Stat(path); err == nil {
			s.ui.LogInfof("CleanupFirmware: Removing canonical BIOS file %s for platform %s", name, platformSlug)
			fileio.Remove(path, s.ui.LogErrorf)
		}
	}
	return nil
}
