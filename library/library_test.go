package library

import (
	"bytes"
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

func (m *MockRomMProvider) DownloadFile(game *types.Game) (io.ReadCloser, string, error) {
	return io.NopCloser(bytes.NewReader([]byte("dummy content"))), "game.sfc", m.Error
}

func (m *MockRomMProvider) GetRom(id uint) (types.Game, error) {
	return m.Game, m.Error
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
	os.WriteFile(romPath, []byte("zip"), 0644)

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
	os.MkdirAll(romDir, 0755)

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
