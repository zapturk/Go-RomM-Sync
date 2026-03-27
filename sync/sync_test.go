package sync

import (
	"bytes"
	"context"
	"go-romm-sync/types"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// MockLibraryProvider implements LibraryProvider
type MockLibraryProvider struct {
	RomDir string
	Game   types.Game
}

func (m *MockLibraryProvider) GetRomDir(game *types.Game) string {
	return m.RomDir
}

func (m *MockLibraryProvider) GetLocalGame(id uint) (types.Game, error) {
	if m.Game.ID != 0 {
		return m.Game, nil
	}
	return types.Game{ID: id}, nil
}

func (m *MockLibraryProvider) GetBiosDir() string {
	return m.RomDir // Use RomDir or a dummy path since we just need it to compile
}

// MockRomMProvider implements RomMProvider
type MockRomMProvider struct {
	Game              types.Game
	UploadErr         error
	DownloadCl        io.ReadCloser
	downloadSaveFunc  func(ctx context.Context, id uint) (io.ReadCloser, string, error)
	downloadStateFunc func(ctx context.Context, id uint) (io.ReadCloser, string, error)
}

func (m *MockRomMProvider) GetRom(id uint) (types.Game, error) { return m.Game, nil }
func (m *MockRomMProvider) RomMUploadSave(id uint, core, filename string, content []byte) error {
	return m.UploadErr
}
func (m *MockRomMProvider) RomMUploadState(id uint, core, filename string, content []byte) error {
	return m.UploadErr
}
func (m *MockRomMProvider) RomMDownloadSave(ctx context.Context, id uint) (reader io.ReadCloser, filename string, err error) {
	if m.downloadSaveFunc != nil {
		return m.downloadSaveFunc(ctx, id)
	}
	return m.DownloadCl, "save.srm", nil
}
func (m *MockRomMProvider) RomMDownloadState(ctx context.Context, id uint) (reader io.ReadCloser, filename string, err error) {
	if m.downloadStateFunc != nil {
		return m.downloadStateFunc(ctx, id)
	}
	return m.DownloadCl, "state.st0", nil
}

// MockUIProvider implements UIProvider
type MockUIProvider struct{}

func (m *MockUIProvider) LogInfof(format string, args ...interface{})      {}
func (m *MockUIProvider) LogErrorf(format string, args ...interface{})     {}
func (m *MockUIProvider) EventsEmit(eventName string, args ...interface{}) {}

func TestValidateAssetPath(t *testing.T) {
	s := &Service{}
	tests := []struct {
		core     string
		filename string
		wantErr  bool
	}{
		{"snes", "save.srm", false},
		{"../snes", "save.srm", false}, // Base will clean it
		{".", "save.srm", true},
		{"snes", "..", true},
	}

	for _, tt := range tests {
		core, file, err := s.ValidateAssetPath(tt.core, tt.filename)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateAssetPath(%s, %s) error = %v, wantErr %v", tt.core, tt.filename, err, tt.wantErr)
		}
		if err == nil {
			if core == "" || file == "" {
				t.Errorf("Expected non-empty core and file")
			}
		}
	}
}

func TestGetSaves_Empty(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "sync_test")
	defer os.RemoveAll(tempDir)

	lib := &MockLibraryProvider{RomDir: tempDir}
	romm := &MockRomMProvider{Game: types.Game{ID: 1}}
	s := New(lib, romm, &MockUIProvider{})

	saves, err := s.GetSaves(1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(saves) != 0 {
		t.Errorf("Expected 0 saves, got %d", len(saves))
	}
}

func TestUploadSave_PathTraversal(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "sync_test")
	defer os.RemoveAll(tempDir)

	lib := &MockLibraryProvider{RomDir: tempDir}
	romm := &MockRomMProvider{Game: types.Game{ID: 1}}
	s := New(lib, romm, &MockUIProvider{})

	err := s.UploadSave(1, "../../etc", "passwd")
	if err == nil {
		t.Errorf("Expected path traversal error")
	}
}

func TestDeleteGameFile(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "sync_test")
	defer os.RemoveAll(tempDir)

	savesDir := filepath.Join(tempDir, "saves", "snes")
	os.MkdirAll(savesDir, 0o755)
	saveFile := filepath.Join(savesDir, "game.srm")
	os.WriteFile(saveFile, []byte("data"), 0o644)

	lib := &MockLibraryProvider{RomDir: tempDir}
	romm := &MockRomMProvider{Game: types.Game{ID: 1}}
	s := New(lib, romm, &MockUIProvider{})

	err := s.DeleteGameFile(1, "saves", "snes", "game.srm")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if _, err := os.Stat(saveFile); !os.IsNotExist(err) {
		t.Errorf("Expected file to be deleted")
	}
}

func TestGetSaves_WithFiles(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "sync_test")
	defer os.RemoveAll(tempDir)

	savesDir := filepath.Join(tempDir, "saves", "snes")
	os.MkdirAll(savesDir, 0o755)
	os.WriteFile(filepath.Join(savesDir, "game.srm"), []byte("data"), 0o644)

	lib := &MockLibraryProvider{RomDir: tempDir}
	romm := &MockRomMProvider{Game: types.Game{ID: 1}}
	s := New(lib, romm, &MockUIProvider{})

	saves, err := s.GetSaves(1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(saves) != 1 {
		t.Errorf("Expected 1 save, got %d", len(saves))
	}
}

func TestDownloadServerAsset(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "sync_test_dl")
	defer os.RemoveAll(tempDir)

	fakeServerData := []byte("server data")

	lib := &MockLibraryProvider{RomDir: tempDir}
	romm := &MockRomMProvider{
		Game: types.Game{ID: 1},
	}
	romm.downloadSaveFunc = func(ctx context.Context, id uint) (io.ReadCloser, string, error) {
		return io.NopCloser(bytes.NewReader(fakeServerData)), "game.srm", nil
	}
	s := New(lib, romm, &MockUIProvider{})

	err := s.DownloadServerSave(1, 123, "snes", "game.srm", "")
	if err != nil {
		t.Fatalf("DownloadServerSave failed: %v", err)
	}

	localPath := filepath.Join(tempDir, "saves", "snes", "game.srm")
	if _, err := os.Stat(localPath); err != nil {
		t.Errorf("Expected local file to be created at %s", localPath)
	}
}

func TestUploadSave_Success(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "sync_test_up")
	defer os.RemoveAll(tempDir)

	savesDir := filepath.Join(tempDir, "saves", "snes")
	os.MkdirAll(savesDir, 0o755)
	saveFile := filepath.Join(savesDir, "game.srm")
	os.WriteFile(saveFile, []byte("data"), 0o644)

	lib := &MockLibraryProvider{RomDir: tempDir}
	romm := &MockRomMProvider{Game: types.Game{ID: 1}}
	s := New(lib, romm, &MockUIProvider{})

	err := s.UploadSave(1, "snes", "game.srm")
	if err != nil {
		t.Fatalf("UploadSave failed: %v", err)
	}
}

func TestGetSaves_DolphinPlatformFilter(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "sync_test_dolphin")
	defer os.RemoveAll(tempDir)

	savesDir := filepath.Join(tempDir, "saves", "dolphin-emu")

	// Create GameCube save
	gcDir := filepath.Join(savesDir, "User", "GC", "USA", "Card A")
	os.MkdirAll(gcDir, 0o755)
	os.WriteFile(filepath.Join(gcDir, "MemoryCardA.USA.raw"), []byte("gc_data"), 0o644)

	// Create Wii save dir
	wiiDir := filepath.Join(savesDir, "User", "Wii")
	os.MkdirAll(filepath.Join(wiiDir, "title"), 0o755)

	lib := &MockLibraryProvider{RomDir: tempDir}
	romm := &MockRomMProvider{}
	s := New(lib, romm, &MockUIProvider{})

	// Test GameCube game (doesn't trigger 'wii' matching)
	lib.Game = types.Game{ID: 1, PlatformSlug: "gamecube"}
	savesGC, err := s.GetSaves(1)
	if err != nil {
		t.Fatalf("Unexpected error for GC check: %v", err)
	}
	if len(savesGC) != 1 || savesGC[0].Name != "MemoryCardA.USA.raw" {
		t.Errorf("Expected 1 GC save, got %v", savesGC)
	}

	// Test Wii game
	lib.Game = types.Game{ID: 2, PlatformSlug: "wii"}
	savesWii, err := s.GetSaves(2)
	if err != nil {
		t.Fatalf("Unexpected error for Wii check: %v", err)
	}
	if len(savesWii) != 1 || savesWii[0].Name != "Wii" {
		t.Errorf("Expected 1 Wii save, got %v", savesWii)
	}
}
