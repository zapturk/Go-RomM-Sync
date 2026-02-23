package launcher

import (
	"context"
	"go-romm-sync/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MockConfigProvider implements ConfigProvider
type MockConfigProvider struct {
	LibraryPath   string
	RetroArchPath string
}

func (m *MockConfigProvider) GetLibraryPath() string {
	return m.LibraryPath
}

func (m *MockConfigProvider) GetRetroArchPath() string {
	return m.RetroArchPath
}

func (m *MockConfigProvider) GetCheevosCredentials() (string, string) {
	return "user", "pass"
}

// MockRomMProvider implements RomMProvider
type MockRomMProvider struct {
	Game  types.Game
	Error error
}

func (m *MockRomMProvider) GetRom(id uint) (types.Game, error) {
	return m.Game, m.Error
}

// MockUIProvider implements UIProvider
type MockUIProvider struct {
	SelectedExe string
	Error       error
}

func (m *MockUIProvider) SelectRetroArchExecutable() (string, error) {
	return m.SelectedExe, m.Error
}
func (m *MockUIProvider) LogInfof(format string, args ...interface{})      {}
func (m *MockUIProvider) LogErrorf(format string, args ...interface{})     {}
func (m *MockUIProvider) EventsEmit(eventName string, args ...interface{}) {}
func (m *MockUIProvider) WindowHide()                                      {}
func (m *MockUIProvider) WindowShow()                                      {}
func (m *MockUIProvider) WindowUnminimise()                                {}
func (m *MockUIProvider) WindowSetAlwaysOnTop(b bool)                      {}

func TestNew(t *testing.T) {
	l := New(&MockConfigProvider{}, &MockRomMProvider{}, &MockUIProvider{})
	if l.config == nil || l.romm == nil || l.ui == nil {
		t.Errorf("Launcher not initialized correctly")
	}
}

func TestSetContext(t *testing.T) {
	l := New(nil, nil, nil)
	ctx := context.Background()
	l.SetContext(ctx)
	if l.ctx != ctx {
		t.Errorf("Context not set correctly")
	}
}

func TestFindRomPath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "launcher_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a dummy ROM file
	romPath := filepath.Join(tempDir, "test.sfc")
	if err := os.WriteFile(romPath, []byte("dummy"), 0644); err != nil {
		t.Fatal(err)
	}

	l := New(nil, nil, nil)
	found := l.findRomPath(tempDir)
	if found != romPath {
		t.Errorf("Expected %s, got %s", romPath, found)
	}

	// Test skip hidden
	hiddenPath := filepath.Join(tempDir, ".hidden.sfc")
	if err := os.WriteFile(hiddenPath, []byte("dummy"), 0644); err != nil {
		t.Fatal(err)
	}
	found = l.findRomPath(tempDir)
	if found != romPath {
		t.Errorf("Expected to still find %s, got %s", romPath, found)
	}
}

func TestPlayRom_NoLibraryPath(t *testing.T) {
	l := New(&MockConfigProvider{LibraryPath: ""}, nil, nil)
	err := l.PlayRom(1)
	if err == nil || err.Error() != "library path is not configured" {
		t.Errorf("Expected library path error, got %v", err)
	}
}

func TestPlayRom_RomNotFound(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "library")
	defer os.RemoveAll(tempDir)

	cfg := &MockConfigProvider{LibraryPath: tempDir}
	romm := &MockRomMProvider{
		Game: types.Game{ID: 1, FullPath: "SNES/Game.sfc"},
	}
	l := New(cfg, romm, &MockUIProvider{})

	err := l.PlayRom(1)
	if err == nil || !contains(err.Error(), "no valid ROM file found") {
		t.Errorf("Expected ROM find error, got %v", err)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// Re-implementing contains because I don't want to import strings just for one test line if I can avoid it in this quick mock, but okay, let's just use it.

func TestPlayRom_RetroArchNotConfigured(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "library")
	defer os.RemoveAll(tempDir)

	romDir := filepath.Join(tempDir, "SNES", "1")
	os.MkdirAll(romDir, 0755)
	os.WriteFile(filepath.Join(romDir, "test.sfc"), []byte("dummy"), 0644)

	cfg := &MockConfigProvider{LibraryPath: tempDir, RetroArchPath: ""}
	romm := &MockRomMProvider{
		Game: types.Game{ID: 1, FullPath: "SNES/Game.sfc"},
	}
	ui := &MockUIProvider{SelectedExe: ""} // User cancelled
	l := New(cfg, romm, ui)

	err := l.PlayRom(1)
	if err == nil || !strings.Contains(err.Error(), "launch cancelled") {
		t.Errorf("Expected launch cancelled error, got %v", err)
	}
}
