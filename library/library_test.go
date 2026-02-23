package library

import (
	"bytes"
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

func (m *MockRomMProvider) DownloadFile(game *types.Game) (reader io.ReadCloser, filename string, err error) {
	return io.NopCloser(bytes.NewReader([]byte("dummy content"))), "game.sfc", m.Error
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

	err := s.DownloadRomToLibrary(1)
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
