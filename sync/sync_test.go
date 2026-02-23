package sync

import (
	"bytes"
	"go-romm-sync/types"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// MockLibraryProvider implements LibraryProvider
type MockLibraryProvider struct {
	RomDir string
}

func (m *MockLibraryProvider) GetRomDir(game *types.Game) string {
	return m.RomDir
}

// MockRomMProvider implements RomMProvider
type MockRomMProvider struct {
	Game       types.Game
	UploadErr  error
	DownloadCl io.ReadCloser
}

func (m *MockRomMProvider) GetRom(id uint) (types.Game, error) { return m.Game, nil }
func (m *MockRomMProvider) RomMUploadSave(id uint, core, filename string, content []byte) error {
	return m.UploadErr
}
func (m *MockRomMProvider) RomMUploadState(id uint, core, filename string, content []byte) error {
	return m.UploadErr
}
func (m *MockRomMProvider) RomMDownloadSave(filePath string) (io.ReadCloser, string, error) {
	return m.DownloadCl, "save.srm", nil
}
func (m *MockRomMProvider) RomMDownloadState(filePath string) (io.ReadCloser, string, error) {
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
	os.MkdirAll(savesDir, 0755)
	saveFile := filepath.Join(savesDir, "game.srm")
	os.WriteFile(saveFile, []byte("data"), 0644)

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
	os.MkdirAll(savesDir, 0755)
	os.WriteFile(filepath.Join(savesDir, "game.srm"), []byte("data"), 0644)

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

	lib := &MockLibraryProvider{RomDir: tempDir}
	romm := &MockRomMProvider{
		Game:       types.Game{ID: 1},
		DownloadCl: io.NopCloser(bytes.NewReader([]byte("server data"))),
	}
	s := New(lib, romm, &MockUIProvider{})

	err := s.DownloadServerSave(1, "remote/path", "snes", "game.srm", "")
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
	os.MkdirAll(savesDir, 0755)
	saveFile := filepath.Join(savesDir, "game.srm")
	os.WriteFile(saveFile, []byte("data"), 0644)

	lib := &MockLibraryProvider{RomDir: tempDir}
	romm := &MockRomMProvider{Game: types.Game{ID: 1}}
	s := New(lib, romm, &MockUIProvider{})

	err := s.UploadSave(1, "snes", "game.srm")
	if err != nil {
		t.Fatalf("UploadSave failed: %v", err)
	}
}
