package sync

import (
	"bytes"
	"encoding/json"
	"go-romm-sync/config"
	"go-romm-sync/library"
	"go-romm-sync/rommsrv"
	"go-romm-sync/types"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

type mockRommConfig struct{}

func (m mockRommConfig) GetRomMHost() string      { return "http://localhost" }
func (m mockRommConfig) GetUsername() string      { return "user" }
func (m mockRommConfig) GetPassword() string      { return "pass" }
func (m mockRommConfig) GetClientToken() string   { return "token" }

type mockTransport struct {
	roundTrip func(*http.Request) (*http.Response, error)
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.roundTrip(req)
}

type MockUIProvider struct{}

func (m *MockUIProvider) LogInfof(format string, args ...interface{})          {}
func (m *MockUIProvider) LogErrorf(format string, args ...interface{})         {}
func (m *MockUIProvider) EventsEmit(eventName string, args ...interface{})     {}

func setupServices(tempDir string, gameData []byte, fileData []byte) (*library.Service, *rommsrv.Service, *config.ConfigManager) {
	cm := config.NewConfigManager()
	cm.ConfigPath = filepath.Join(tempDir, "config.json")
	cm.Config = &types.AppConfig{LibraryPath: tempDir}

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
				Body:       io.NopCloser(bytes.NewReader(fileData)),
			}, nil
		},
	}

	libSrv := library.New(cm, rommSrv, &MockUIProvider{})
	return libSrv, rommSrv, cm
}

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
	tempDir, err := os.MkdirTemp("", "sync_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	game := types.Game{ID: 1, FullPath: "snes/game.sfc"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)
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
	tempDir, err := os.MkdirTemp("", "sync_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	game := types.Game{ID: 1, FullPath: "snes/game.sfc"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)
	s := New(lib, romm, &MockUIProvider{})

	err = s.UploadSave(1, "../../etc", "passwd")
	if err == nil {
		t.Errorf("Expected path traversal error")
	}
}

func TestDeleteGameFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	romDir := filepath.Join(tempDir, "snes", "1")
	savesDir := filepath.Join(romDir, "saves", "snes")
	if err := os.MkdirAll(savesDir, 0o755); err != nil {
		t.Fatalf("failed to create saves dir: %v", err)
	}
	saveFile := filepath.Join(savesDir, "game.srm")
	if err := os.WriteFile(saveFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write save file: %v", err)
	}

	game := types.Game{ID: 1, FullPath: "snes/game.sfc"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)
	s := New(lib, romm, &MockUIProvider{})

	err = s.DeleteGameFile(1, "saves", "snes", "game.srm")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if _, err := os.Stat(saveFile); !os.IsNotExist(err) {
		t.Errorf("Expected file to be deleted")
	}
}

func TestGetSaves_WithFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	romDir := filepath.Join(tempDir, "snes", "1")
	savesDir := filepath.Join(romDir, "saves", "snes")
	if err := os.MkdirAll(savesDir, 0o755); err != nil {
		t.Fatalf("failed to create saves dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(savesDir, "game.srm"), []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write save file: %v", err)
	}

	game := types.Game{ID: 1, FullPath: "snes/game.sfc"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)
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
	tempDir, err := os.MkdirTemp("", "sync_test_dl")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fakeServerData := []byte("server data")

	game := types.Game{ID: 1, FullPath: "snes/game.sfc"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, fakeServerData)
	s := New(lib, romm, &MockUIProvider{})

	err = s.DownloadServerSave(1, 123, "snes", "game.srm", "")
	if err != nil {
		t.Fatalf("DownloadServerSave failed: %v", err)
	}

	localPath := filepath.Join(tempDir, "snes", "1", "saves", "snes", "game.srm")
	if _, err := os.Stat(localPath); err != nil {
		t.Errorf("Expected local file to be created at %s", localPath)
	}
}

func TestUploadSave_Success(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test_up")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	romDir := filepath.Join(tempDir, "snes", "1")
	savesDir := filepath.Join(romDir, "saves", "snes")
	if err := os.MkdirAll(savesDir, 0o755); err != nil {
		t.Fatalf("failed to create saves dir: %v", err)
	}
	saveFile := filepath.Join(savesDir, "game.srm")
	if err := os.WriteFile(saveFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write save file: %v", err)
	}

	game := types.Game{ID: 1, FullPath: "snes/game.sfc"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)

	// Mock upload endpoint returning HTTP 200
	romm.GetClient().APIClient.Transport = &mockTransport{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			if req.Method == "POST" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte("{}"))),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(gameData)),
			}, nil
		},
	}

	s := New(lib, romm, &MockUIProvider{})

	err = s.UploadSave(1, "snes", "game.srm")
	if err != nil {
		t.Fatalf("UploadSave failed: %v", err)
	}
}

func TestGetSaves_DolphinPlatformFilter(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test_dolphin")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	gameGC := types.Game{ID: 1, PlatformSlug: "gamecube", FullPath: "gamecube/game.iso"}
	gameWii := types.Game{ID: 2, PlatformSlug: "wii", FullPath: "wii/game.wbfs"}
	gameGCData, _ := json.Marshal(gameGC)

	lib, romm, _ := setupServices(tempDir, gameGCData, nil)
	s := New(lib, romm, &MockUIProvider{})

	// Create GameCube save
	gcDir := filepath.Join(tempDir, "gamecube", "1", "saves", "dolphin-emu", "User", "GC", "USA", "Card A")
	if err := os.MkdirAll(gcDir, 0o755); err != nil {
		t.Fatalf("failed to create GC saves dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gcDir, "MemoryCardA.USA.raw"), []byte("gc_data"), 0o644); err != nil {
		t.Fatalf("failed to write GC save file: %v", err)
	}

	// Create Wii save dir
	wiiDir := filepath.Join(tempDir, "wii", "2", "saves", "dolphin-emu", "User", "Wii")
	if err := os.MkdirAll(filepath.Join(wiiDir, "title"), 0o755); err != nil {
		t.Fatalf("failed to create Wii title dir: %v", err)
	}

	// Test GameCube game
	savesGC, err := s.GetSaves(1)
	if err != nil {
		t.Fatalf("Unexpected error for GC check: %v", err)
	}
	if len(savesGC) != 1 || savesGC[0].Name != "MemoryCardA.USA.raw" {
		t.Errorf("Expected 1 GC save, got %v", savesGC)
	}

	// Mock server to return Wii game details on GetRom
	gameWiiData, _ := json.Marshal(gameWii)
	romm.GetClient().APIClient.Transport = &mockTransport{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(gameWiiData)),
			}, nil
		},
	}

	// Test Wii game
	savesWii, err := s.GetSaves(2)
	if err != nil {
		t.Fatalf("Unexpected error for Wii check: %v", err)
	}
	if len(savesWii) != 1 || savesWii[0].Name != "Wii" {
		t.Errorf("Expected 1 Wii save, got %v", savesWii)
	}
}

func TestGetSaves_PPSSPP(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test_ppsspp")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	game := types.Game{ID: 954, PlatformSlug: "psp", FullPath: "psp/game.iso"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)
	s := New(lib, romm, &MockUIProvider{})

	// User's path structure: saves/PPSSPP/PSP/SAVEDATA/ULUS...
	saveName := "ULUS10374SO10000"
	savesDir := filepath.Join(tempDir, "psp", "954", "saves", "PPSSPP", "PSP", "SAVEDATA", saveName)
	if err := os.MkdirAll(savesDir, 0o755); err != nil {
		t.Fatalf("failed to create PPSSPP saves dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(savesDir, "PARAM.SFO"), []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write save file: %v", err)
	}

	saves, err := s.GetSaves(954)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(saves) != 1 || saves[0].Name != saveName {
		t.Errorf("Expected 1 PPSSPP save %s, got %v", saveName, saves)
	}
}

func TestPrepareAssetPath_PPSSPP(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test_ppsspp_path")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	game := types.Game{ID: 954, PlatformSlug: "psp", FullPath: "psp/game.iso"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)
	s := New(lib, romm, &MockUIProvider{})

	destPath, err := s.prepareAssetPath(&game, "PPSSPP", "ULUS10374SO10000", "saves")
	if err != nil {
		t.Fatalf("prepareAssetPath failed: %v", err)
	}

	expected := filepath.Join(tempDir, "psp", "954", "saves", "PPSSPP", "PSP", "SAVEDATA", "ULUS10374SO10000")
	if destPath != expected {
		t.Errorf("Expected path %s, got %s", expected, destPath)
	}
}
