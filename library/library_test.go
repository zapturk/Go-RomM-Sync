package library

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go-romm-sync/config"
	"go-romm-sync/rommsrv"
	"go-romm-sync/types"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

type mockRommConfig struct{}

func (m mockRommConfig) GetRomMHost() string    { return "http://localhost" }
func (m mockRommConfig) GetUsername() string    { return "user" }
func (m mockRommConfig) GetPassword() string    { return "pass" }
func (m mockRommConfig) GetClientToken() string { return "token" }

type mockTransport struct {
	roundTrip func(*http.Request) (*http.Response, error)
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.roundTrip(req)
}

type MockUIProvider struct {
	LastEvent string
}

func (m *MockUIProvider) LogInfof(format string, args ...interface{})  {}
func (m *MockUIProvider) LogErrorf(format string, args ...interface{}) {}
func (m *MockUIProvider) EventsEmit(eventName string, args ...interface{}) {
	m.LastEvent = eventName
}

func TestNew(t *testing.T) {
	cm := config.NewConfigManager()
	romm := rommsrv.New(mockRommConfig{})
	s := New(cm, romm, &MockUIProvider{})
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
	tempDir, _ := os.MkdirTemp("", "library_test_getromdir")
	defer os.RemoveAll(tempDir)

	cm := config.NewConfigManager()
	cm.ConfigPath = filepath.Join(tempDir, "config.json")
	cm.Config = &types.AppConfig{LibraryPath: "/base"}

	s := New(cm, nil, nil)
	game := &types.Game{ID: 1, FullPath: "SNES/Game.sfc"}

	dir := s.GetRomDir(game)
	expected := filepath.Join("/base", "SNES", "1")
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
	found := s.findRomPath(tempDir, nil)
	if found != romPath {
		t.Errorf("Expected %s, got %s", romPath, found)
	}
}

func TestDeleteRom(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "library")
	defer os.RemoveAll(tempDir)

	romDir := filepath.Join(tempDir, "SNES", "1")
	os.MkdirAll(romDir, 0o755)

	cm := config.NewConfigManager()
	cm.ConfigPath = filepath.Join(tempDir, "config.json")
	cm.Config = &types.AppConfig{LibraryPath: tempDir}

	game := types.Game{ID: 1, FullPath: "SNES/Game.sfc"}
	gameData, _ := json.Marshal(game)

	rommSrv := rommsrv.New(mockRommConfig{})
	rommSrv.GetClient().APIClient.Transport = &mockTransport{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(gameData)),
			}, nil
		},
	}

	ui := &MockUIProvider{}
	s := New(cm, rommSrv, ui)

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

	cm := config.NewConfigManager()
	cm.ConfigPath = filepath.Join(tempDir, "config.json")
	cm.Config = &types.AppConfig{LibraryPath: tempDir}

	game := types.Game{ID: 1, FullPath: "SNES/Game.sfc", FileSize: 100}
	gameData, _ := json.Marshal(game)

	rommSrv := rommsrv.New(mockRommConfig{})
	rommSrv.GetClient().APIClient.Transport = &mockTransport{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(gameData)),
			}, nil
		},
	}
	rommSrv.GetClient().FileClient.Transport = &mockTransport{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("dummy content"))),
			}, nil
		},
	}

	ui := &MockUIProvider{}
	s := New(cm, rommSrv, ui)

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

	cm := config.NewConfigManager()
	cm.ConfigPath = filepath.Join(tempDir, "config.json")
	cm.Config = &types.AppConfig{LibraryPath: tempDir}

	game1 := types.Game{ID: 1, FullPath: "SNES/Game.sfc"}
	game2 := types.Game{ID: 2, FullPath: "SNES/Game2.sfc"}

	rommSrv := rommsrv.New(mockRommConfig{})
	rommSrv.GetClient().APIClient.Transport = &mockTransport{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			var respData []byte
			if req.URL.Path == "/api/roms/1" {
				respData, _ = json.Marshal(game1)
			} else {
				respData, _ = json.Marshal(game2)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(respData)),
			}, nil
		},
	}

	s := New(cm, rommSrv, &MockUIProvider{})

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

func TestDownloadRomToLibrary_Cleanup(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "library_cleanup_test")
	defer os.RemoveAll(tempDir)

	cm := config.NewConfigManager()
	cm.ConfigPath = filepath.Join(tempDir, "config.json")
	cm.Config = &types.AppConfig{LibraryPath: tempDir}

	game := types.Game{ID: 1, FullPath: "SNES/Game.sfc", FileSize: 100}
	gameData, _ := json.Marshal(game)

	rommSrv := rommsrv.New(mockRommConfig{})
	rommSrv.GetClient().APIClient.Transport = &mockTransport{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(gameData)),
			}, nil
		},
	}
	rommSrv.GetClient().FileClient.Transport = &mockTransport{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(&errorReader{}),
			}, nil
		},
	}

	ui := &MockUIProvider{}
	s := New(cm, rommSrv, ui)

	err := s.DownloadRomToLibrary(context.Background(), 1)
	if err == nil {
		t.Errorf("Expected error from failed download, got nil")
	}

	destPath := filepath.Join(tempDir, "SNES", "1", "Game.sfc")
	if _, err := os.Stat(destPath); err == nil {
		t.Errorf("Expected partial ROM file at %s to be deleted on failure", destPath)
	}
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

	cm := config.NewConfigManager()
	cm.ConfigPath = filepath.Join(tempDir, "config.json")
	cm.Config = &types.AppConfig{LibraryPath: tempDir}

	rommSrv := rommsrv.New(mockRommConfig{})
	ui := &MockUIProvider{}
	s := New(cm, rommSrv, ui)

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

func TestMigrateLibrary(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "library_migration_test")
	defer os.RemoveAll(tempDir)

	cm := config.NewConfigManager()
	cm.ConfigPath = filepath.Join(tempDir, "config.json")
	cm.Config = &types.AppConfig{
		LibraryPath:       tempDir,
		UsePlatformFolder: false,
	}

	ui := &MockUIProvider{}
	s := New(cm, nil, ui)

	game := types.Game{
		ID:           99,
		Title:        "Test Game",
		FullPath:     "snes/Test Game.sfc",
		PlatformSlug: "snes",
	}

	// 1. Create a game in ID folder format
	idDir := filepath.Join(tempDir, "snes", "99")
	os.MkdirAll(idDir, 0o755)
	romFile := filepath.Join(idDir, "Test Game.sfc")
	os.WriteFile(romFile, []byte("fake snes rom"), 0o644)
	
	// Create mock metadata
	s.SaveMetadata(&game)

	// Save a fake save file
	saveDir := filepath.Join(idDir, "saves", "snes9x")
	os.MkdirAll(saveDir, 0o755)
	saveFile := filepath.Join(saveDir, "Test Game.srm")
	os.WriteFile(saveFile, []byte("fake save"), 0o644)

	// 2. Migrate to Platform Folder format
	err := s.MigrateLibrary(true)
	if err != nil {
		t.Fatalf("Migration to platform folder failed: %v", err)
	}

	// Verify ID dir is gone
	if _, err := os.Stat(idDir); !os.IsNotExist(err) {
		t.Errorf("Expected ID directory to be removed")
	}

	// Verify ROM is now in platform folder
	platformRomFile := filepath.Join(tempDir, "snes", "Test Game.sfc")
	if _, err := os.Stat(platformRomFile); err != nil {
		t.Errorf("Expected ROM file to exist at %s: %v", platformRomFile, err)
	}

	// Verify metadata is now metadata_99.json in platform folder
	platformMetaFile := filepath.Join(tempDir, "snes", "metadata_99.json")
	if _, err := os.Stat(platformMetaFile); err != nil {
		t.Errorf("Expected metadata file to exist at %s: %v", platformMetaFile, err)
	}

	// Verify save is now in platform/saves/snes9x/Test Game.srm
	platformSaveFile := filepath.Join(tempDir, "snes", "saves", "snes9x", "Test Game.srm")
	if _, err := os.Stat(platformSaveFile); err != nil {
		t.Errorf("Expected save file to exist at %s: %v", platformSaveFile, err)
	}

	// Update Config to UsePlatformFolder: true for scanning logic
	cm.Config.UsePlatformFolder = true

	// 3. Migrate back to ID Folder format
	err = s.MigrateLibrary(false)
	if err != nil {
		t.Fatalf("Migration back to ID folder failed: %v", err)
	}

	// Verify platform ROM and metadata and saves are gone or moved back
	if _, err := os.Stat(platformRomFile); !os.IsNotExist(err) {
		t.Errorf("Expected platform ROM file to be moved/removed")
	}
	if _, err := os.Stat(platformMetaFile); !os.IsNotExist(err) {
		t.Errorf("Expected platform metadata file to be moved/removed")
	}
	if _, err := os.Stat(platformSaveFile); !os.IsNotExist(err) {
		t.Errorf("Expected platform save file to be moved/removed")
	}

	// Verify it's back in the ID folder
	if _, err := os.Stat(romFile); err != nil {
		t.Errorf("Expected ROM file to be restored to %s", romFile)
	}
	if _, err := os.Stat(filepath.Join(idDir, "metadata.json")); err != nil {
		t.Errorf("Expected metadata file to be restored to %s", filepath.Join(idDir, "metadata.json"))
	}
	if _, err := os.Stat(saveFile); err != nil {
		t.Errorf("Expected save file to be restored to %s", saveFile)
	}
}

func TestCleanupOrphanedRoms(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "library_cleanup_test")
	defer os.RemoveAll(tempDir)

	cm := config.NewConfigManager()
	cm.ConfigPath = filepath.Join(tempDir, "config.json")
	cm.Config = &types.AppConfig{
		LibraryPath:       tempDir,
		UsePlatformFolder: false,
	}

	ui := &MockUIProvider{}
	s := New(cm, nil, ui)

	game := types.Game{
		ID:           42,
		Title:        "Tracked Game",
		FullPath:     "nes/tracked.nes",
		PlatformSlug: "nes",
	}

	// 1. Create tracked game files
	idDir := filepath.Join(tempDir, "nes", "42")
	os.MkdirAll(idDir, 0o755)
	romFile := filepath.Join(idDir, "tracked.nes")
	os.WriteFile(romFile, []byte("fake rom"), 0o644)
	s.SaveMetadata(&game)

	// 2. Create orphaned files/folders
	orphanDir := filepath.Join(tempDir, "nes", "999")
	os.MkdirAll(orphanDir, 0o755)
	orphanRomFile := filepath.Join(orphanDir, "orphaned_game.nes")
	os.WriteFile(orphanRomFile, []byte("fake orphan rom"), 0o644)

	// Create an orphaned file directly in platform folder
	orphanFlatFile := filepath.Join(tempDir, "nes", "orphaned_flat.nes")
	os.WriteFile(orphanFlatFile, []byte("fake flat orphan"), 0o644)

	// Run cleanup
	files, err := s.ScanOrphanedRoms()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	count, err := s.DeleteOrphanedRoms(files)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected to delete 2 orphaned files, got %d", count)
	}

	// Tracked files should remain
	if _, err := os.Stat(romFile); err != nil {
		t.Errorf("Tracked ROM was deleted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(idDir, "metadata.json")); err != nil {
		t.Errorf("Tracked metadata was deleted: %v", err)
	}

	// Orphaned files should be gone
	if _, err := os.Stat(orphanRomFile); !os.IsNotExist(err) {
		t.Errorf("Orphaned ROM was not deleted")
	}
	if _, err := os.Stat(orphanFlatFile); !os.IsNotExist(err) {
		t.Errorf("Orphaned flat ROM was not deleted")
	}
	if _, err := os.Stat(orphanDir); !os.IsNotExist(err) {
		t.Errorf("Orphaned directory was not deleted")
	}
}
