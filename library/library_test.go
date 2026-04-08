package library

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"go-romm-sync/types"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// MockConfigProvider implements ConfigProvider
type MockConfigProvider struct {
	LibraryPath string
}

func (m *MockConfigProvider) GetLibraryPath() string {
	return m.LibraryPath
}

func (m *MockConfigProvider) SaveDefaultLibraryPath(path string) error {
	m.LibraryPath = path
	return nil
}

// MockRomMProvider implements RomMProvider
type MockRomMProvider struct {
	Game  types.Game
	Error error
}

func (m *MockRomMProvider) DownloadFile(ctx context.Context, game *types.Game) (reader io.ReadCloser, filename string, err error) {
	return io.NopCloser(bytes.NewReader([]byte("dummy content"))), "Game.sfc", m.Error
}

func (m *MockRomMProvider) GetRom(id uint) (types.Game, error) {
	if m.Error != nil {
		return types.Game{}, m.Error
	}
	if m.Game.ID != id {
		return types.Game{}, fmt.Errorf("not found")
	}
	return m.Game, nil
}

func (m *MockRomMProvider) GetFirmware(platformID uint) ([]types.Firmware, error) {
	return nil, m.Error
}

func (m *MockRomMProvider) GetRomDownloadStatus(id uint) (bool, error) {
	return id == 1 && m.Error == nil, nil
}

func (m *MockRomMProvider) DownloadFirmwareContent(ctx context.Context, id uint, fileName string) (io.ReadCloser, string, error) {
	return io.NopCloser(bytes.NewReader([]byte("dummy firmware"))), fileName, m.Error
}

// MockUIProvider implements UIProvider
type MockUIProvider struct {
	LastEvent string
}

func (m *MockUIProvider) LogInfof(format string, args ...interface{})  {}
func (m *MockUIProvider) LogErrorf(format string, args ...interface{}) {}
func (m *MockUIProvider) EventsEmit(eventName string, args ...interface{}) {
	m.LastEvent = eventName
}

func TestNew(t *testing.T) {
	s := New(&MockConfigProvider{}, &MockRomMProvider{}, &MockUIProvider{})
	if s.config == nil || s.romm == nil || s.ui == nil {
		t.Errorf("Service not initialized correctly")
	}
}

func TestProgressWriter_Write(t *testing.T) {
	ui := &MockUIProvider{}
	pw := &ProgressWriter{
		Total:  100,
		GameID: 1,
		UI:     ui,
	}

	_, err := pw.Write([]byte("12345"))
	if err != nil {
		t.Fatal(err)
	}

	if ui.LastEvent != "download-progress" {
		t.Errorf("Expected download-progress event, got %s", ui.LastEvent)
	}
	if pw.Downloaded != 5 {
		t.Errorf("Expected 5 bytes downloaded, got %d", pw.Downloaded)
	}
}

func TestGetRomDir(t *testing.T) {
	cfg := &MockConfigProvider{LibraryPath: "/base"}
	s := New(cfg, nil, nil)
	game := &types.Game{ID: 1, FullPath: "SNES/Game.sfc"}

	dir := s.GetRomDir(game)
	expected := filepath.Join(cfg.GetLibraryPath(), "SNES", "1")
	if dir != expected {
		t.Errorf("Expected %s, got %s", expected, dir)
	}
}

func TestFindRomPath(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "library_test")
	defer os.RemoveAll(tempDir)

	romPath := filepath.Join(tempDir, "game.zip")
	os.WriteFile(romPath, []byte("zip"), 0o644)

	s := New(nil, nil, nil)
	found := s.findRomPath(tempDir)
	if found != romPath {
		t.Errorf("Expected %s, got %s", romPath, found)
	}
}

func TestDeleteRom(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "library")
	defer os.RemoveAll(tempDir)

	romDir := filepath.Join(tempDir, "SNES", "1")
	os.MkdirAll(romDir, 0o755)

	cfg := &MockConfigProvider{LibraryPath: tempDir}
	romm := &MockRomMProvider{
		Game: types.Game{ID: 1, FullPath: "SNES/Game.sfc"},
	}
	ui := &MockUIProvider{}
	s := New(cfg, romm, ui)

	err := s.DeleteRom(1)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if _, err := os.Stat(romDir); !os.IsNotExist(err) {
		t.Errorf("Expected ROM directory to be deleted")
	}
}

func TestDownloadRomToLibrary(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "library_dl")
	defer os.RemoveAll(tempDir)

	cfg := &MockConfigProvider{LibraryPath: tempDir}
	romm := &MockRomMProvider{
		Game: types.Game{ID: 1, FullPath: "SNES/Game.sfc", FileSize: 100},
	}
	ui := &MockUIProvider{}
	s := New(cfg, romm, ui)

	err := s.DownloadRomToLibrary(context.Background(), 1)
	if err != nil {
		t.Fatalf("DownloadRomToLibrary failed: %v", err)
	}

	destPath := filepath.Join(tempDir, "SNES", "1", "Game.sfc")
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("Expected ROM file to be created at %s", destPath)
	}
}

func TestGetRomDownloadStatus(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "library_status")
	defer os.RemoveAll(tempDir)

	romDir := filepath.Join(tempDir, "SNES", "1")
	os.MkdirAll(romDir, 0o755)
	os.WriteFile(filepath.Join(romDir, "game.sfc"), []byte("data"), 0o644)

	cfg := &MockConfigProvider{LibraryPath: tempDir}
	romm := &MockRomMProvider{
		Game: types.Game{ID: 1, FullPath: "SNES/Game.sfc"},
	}
	s := New(cfg, romm, &MockUIProvider{})

	status, err := s.GetRomDownloadStatus(1)
	if err != nil {
		t.Fatalf("GetRomDownloadStatus failed: %v", err)
	}
	if !status {
		t.Errorf("Expected status true, got false")
	}

	// Test case where it's not downloaded
	status, _ = s.GetRomDownloadStatus(2)
	if status {
		t.Errorf("Expected status false for missing game")
	}
}

func TestDownloadFirmware(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "library_fw")
	defer os.RemoveAll(tempDir)

	cfg := &MockConfigProvider{LibraryPath: tempDir}
	// Sega CD (U) BIOS MD5
	md5 := "baca1df271d7c11fe50087c0358f4eb5"
	fw := &types.Firmware{
		ID:       1,
		FileName: "scd_v2.21.bin",
		MD5Hash:  md5,
	}

	romm := &MockRomMProvider{}
	ui := &MockUIProvider{}
	s := New(cfg, romm, ui)

	err := s.DownloadFirmware("segacd", fw)
	if err != nil {
		t.Fatalf("DownloadFirmware failed: %v", err)
	}

	// Should be renamed to bios_CD_U.bin based on MD5
	destPath := filepath.Join(tempDir, "bios", "bios_CD_U.bin")
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("Expected BIOS file to be renamed to bios_CD_U.bin, but not found at %s", destPath)
	}
}

func TestDownloadFirmware_Compressed(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "library_fw_zip")
	defer os.RemoveAll(tempDir)

	cfg := &MockConfigProvider{LibraryPath: tempDir}

	// Create a dummy ZIP file
	zipBuffer := new(bytes.Buffer)
	zw := zip.NewWriter(zipBuffer)

	f, _ := zw.Create("internal_bios.bin")
	f.Write([]byte("some bios content"))
	zw.Close()

	fw := &types.Firmware{
		ID:       1,
		FileName: "bios_package.zip",
	}

	// Mock RomM to return the zip content
	romm := &MockRomMProviderWithContent{
		Content: zipBuffer.Bytes(),
		Name:    "bios_package.zip",
	}
	ui := &MockUIProvider{}
	s := New(cfg, romm, ui)

	err := s.DownloadFirmware("nds", fw)
	if err != nil {
		t.Fatalf("DownloadFirmware failed: %v", err)
	}

	// Should be extracted to bios/internal_bios.bin
	destPath := filepath.Join(tempDir, "bios", "internal_bios.bin")
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("Expected extracted BIOS file to be found at %s", destPath)
	}
}

type MockRomMProviderWithContent struct {
	MockRomMProvider
	Content []byte
	Name    string
}

func (m *MockRomMProviderWithContent) DownloadFirmwareContent(ctx context.Context, id uint, fileName string) (io.ReadCloser, string, error) {
	return io.NopCloser(bytes.NewReader(m.Content)), m.Name, nil
}

func TestCleanupFirmware(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "library_cleanup")
	defer os.RemoveAll(tempDir)

	cfg := &MockConfigProvider{LibraryPath: tempDir}
	s := New(cfg, nil, &MockUIProvider{})

	biosDir := filepath.Join(tempDir, "bios")
	os.MkdirAll(biosDir, 0o755)

	// Create a dummy BIOS file for Sega CD
	biosPath := filepath.Join(biosDir, "bios_CD_U.bin")
	os.WriteFile(biosPath, []byte("fake bios"), 0o644)

	// Create a dummy BIOS file for PS1 (should stay)
	ps1Path := filepath.Join(biosDir, "scph5501.bin")
	os.WriteFile(ps1Path, []byte("fake ps1 bios"), 0o644)

	// Cleanup for Sega CD
	err := s.CleanupFirmware("segacd")
	if err != nil {
		t.Fatalf("CleanupFirmware failed: %v", err)
	}

	// Sega CD BIOS should be gone
	if _, err := os.Stat(biosPath); !os.IsNotExist(err) {
		t.Errorf("Expected Sega CD BIOS to be deleted")
	}

	// PS1 BIOS should still be there
	if _, err := os.Stat(ps1Path); err != nil {
		t.Errorf("Expected PS1 BIOS to still exist")
	}
}

func TestDownloadRomToLibrary_Cleanup(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "library_cleanup_test")
	defer os.RemoveAll(tempDir)

	cfg := &MockConfigProvider{LibraryPath: tempDir}
	romm := &MockRomMProviderWithError{
		MockRomMProvider: MockRomMProvider{
			Game: types.Game{ID: 1, FullPath: "SNES/Game.sfc", FileSize: 100},
		},
	}
	ui := &MockUIProvider{}
	s := New(cfg, romm, ui)

	err := s.DownloadRomToLibrary(context.Background(), 1)
	if err == nil {
		t.Errorf("Expected error from failed download, got nil")
	}

	destPath := filepath.Join(tempDir, "SNES", "1", "Game.sfc")
	if _, err := os.Stat(destPath); err == nil {
		t.Errorf("Expected partial ROM file at %s to be deleted on failure", destPath)
	}
}

type MockRomMProviderWithError struct {
	MockRomMProvider
}

func (m *MockRomMProviderWithError) DownloadFile(ctx context.Context, game *types.Game) (io.ReadCloser, string, error) {
	return io.NopCloser(&errorReader{}), "Game.sfc", nil
}

type errorReader struct {
	read bool
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	if r.read {
		return 0, fmt.Errorf("simulated read error")
	}
	// Return some data first
	copy(p, "partial data")
	r.read = true
	return 12, nil
}

func TestPostDownloadProcessing_ExtractionInterference(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "library_interference_test")
	defer os.RemoveAll(tempDir)

	// Create a dummy archive that looks like it could contain both
	// We'll use a real zip file to make archive.ExtractCueBin and ExtractGameCube happy
	archivePath := filepath.Join(tempDir, "game.zip")
	zipFile, _ := os.Create(archivePath)
	zw := zip.NewWriter(zipFile)

	// Add files that satisfy ExtractCueBin
	f1, _ := zw.Create("game.cue")
	f1.Write([]byte("FILE \"game.bin\" BINARY\n  TRACK 01 MODE1/2352\n    INDEX 01 00:00:00"))
	f2, _ := zw.Create("game.bin")
	f2.Write([]byte("fake bin content"))

	// Add a file that satisfies ExtractGameCube (though normally it's one or the other,
	// we want to test that the logic doesn't crash if both are checked)
	f3, _ := zw.Create("game.rvz")
	f3.Write([]byte("fake rvz content"))

	zw.Close()
	zipFile.Close()

	cfg := &MockConfigProvider{LibraryPath: tempDir}
	romm := &MockRomMProvider{}
	ui := &MockUIProvider{}
	s := New(cfg, romm, ui)

	game := &types.Game{ID: 1, FullPath: "GameCube/game.zip"}
	destDir := filepath.Join(tempDir, "GameCube", "1")
	os.MkdirAll(destDir, 0o755)

	// We need to move the archive to where postDownloadProcessing expects it
	finalArchivePath := filepath.Join(destDir, "game.zip")
	os.Rename(archivePath, finalArchivePath)

	err := s.postDownloadProcessing(1, game, finalArchivePath, destDir)
	if err != nil {
		t.Fatalf("postDownloadProcessing failed: %v", err)
	}

	// Verify that the archive was removed
	if _, err := os.Stat(finalArchivePath); !os.IsNotExist(err) {
		t.Errorf("Expected archive to be removed after successful extraction")
	}

	// Verify that files were extracted (at least .cue/.bin or GameCube)
	if _, err := os.Stat(filepath.Join(destDir, "game.cue")); err != nil {
		t.Errorf("Expected game.cue to be extracted")
	}
	if _, err := os.Stat(filepath.Join(destDir, "game.bin")); err != nil {
		t.Errorf("Expected game.bin to be extracted")
	}
}
