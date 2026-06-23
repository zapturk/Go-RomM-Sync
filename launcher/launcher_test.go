package launcher

import (
	"context"
	"go-romm-sync/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockProvider implements all methods needed by Launcher for testing.
type mockProvider struct {
	LibraryPath   string
	RetroArchPath string
	Game          types.Game
	Error         error
	SelectedExe   string
	UiError       error
	LastSavedCore string
	ResolvedSlug  string
}

func (m *mockProvider) GetLibraryPath() string                  { return m.LibraryPath }
func (m *mockProvider) GetRetroArchPath() string                { return m.RetroArchPath }
func (m *mockProvider) GetCheevosCredentials() (string, string) { return "user", "pass" }
func (m *mockProvider) GetBiosDir() string                      { return "" }
func (m *mockProvider) GetRom(id uint) (types.Game, error)      { return m.Game, m.Error }
func (m *mockProvider) SaveLastUsedCore(platformSlug, coreName string) error {
	m.LastSavedCore = coreName
	return nil
}
func (m *mockProvider) GetResolvedPlatformSlug(game *types.Game) string  { return m.ResolvedSlug }
func (m *mockProvider) SelectRetroArchExecutable() (string, error)       { return m.SelectedExe, m.UiError }
func (m *mockProvider) LogInfof(format string, args ...interface{})      {}
func (m *mockProvider) LogErrorf(format string, args ...interface{})     {}
func (m *mockProvider) EventsEmit(eventName string, args ...interface{}) {}
func (m *mockProvider) WindowHide()                                      {}
func (m *mockProvider) WindowShow()                                      {}
func (m *mockProvider) WindowUnminimise()                                {}
func (m *mockProvider) WindowSetAlwaysOnTop(b bool)                      {}

func TestNew(t *testing.T) {
	l := New(&mockProvider{})
	if l.app == nil {
		t.Errorf("Launcher not initialized correctly")
	}
}

func TestSetContext(t *testing.T) {
	l := New(&mockProvider{})
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
	if err := os.WriteFile(romPath, []byte("dummy"), 0o644); err != nil {
		t.Fatal(err)
	}

	l := New(&mockProvider{})
	game := &types.Game{FullPath: "test.sfc"}
	found := l.findRomPath(game, tempDir)
	if found != romPath {
		t.Errorf("Expected %s, got %s", romPath, found)
	}

	// Test skip hidden
	hiddenPath := filepath.Join(tempDir, ".hidden.sfc")
	if err := os.WriteFile(hiddenPath, []byte("dummy"), 0o644); err != nil {
		t.Fatal(err)
	}
	found = l.findRomPath(game, tempDir)
	if found != romPath {
		t.Errorf("Expected to still find %s, got %s", romPath, found)
	}
}

func TestPlayRom_NoLibraryPath(t *testing.T) {
	l := New(&mockProvider{LibraryPath: ""})
	err := l.PlayRom(1)
	if err == nil || err.Error() != "library path is not configured" {
		t.Errorf("Expected library path error, got %v", err)
	}
}

func TestPlayRom_RomNotFound(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "library")
	defer os.RemoveAll(tempDir)

	romDir := filepath.Join(tempDir, "SNES", "1")
	os.MkdirAll(romDir, 0o755)
	// Don't create test.sfc — that's why ROM not found should error.

	p := &mockProvider{
		LibraryPath:   tempDir,
		RetroArchPath: "/some/path/retroarch",
		Game:          types.Game{ID: 1, FullPath: "SNES/Game.sfc"},
	}
	l := New(p)

	err := l.PlayRom(1)
	if err == nil || !contains(err.Error(), "no valid ROM file found") {
		t.Errorf("Expected ROM find error, got %v", err)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// Re-implementing contains to avoid importing strings just for one test line.

func TestPlayRom_RetroArchNotConfigured(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "library")
	defer os.RemoveAll(tempDir)

	romDir := filepath.Join(tempDir, "SNES", "1")
	os.MkdirAll(romDir, 0o755)
	os.WriteFile(filepath.Join(romDir, "test.sfc"), []byte("dummy"), 0o644)

	p := &mockProvider{
		LibraryPath:   tempDir,
		RetroArchPath: "",
		Game:          types.Game{ID: 1, FullPath: "SNES/Game.sfc"},
		SelectedExe:   "", // User cancelled
	}
	l := New(p)

	err := l.PlayRom(1)
	if err == nil || !strings.Contains(err.Error(), "launch cancelled") {
		t.Errorf("Expected launch cancelled error, got %v", err)
	}
}
