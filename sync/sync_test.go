package sync

import (
	"bytes"
	"encoding/json"
	"go-romm-sync/config"
	"go-romm-sync/constants"
	"go-romm-sync/library"
	"go-romm-sync/rommsrv"
	"go-romm-sync/types"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
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

type MockUIProvider struct{}

func (m *MockUIProvider) LogInfof(format string, args ...interface{})      {}
func (m *MockUIProvider) LogErrorf(format string, args ...interface{})     {}
func (m *MockUIProvider) EventsEmit(eventName string, args ...interface{}) {}

func setupServices(tempDir string, gameData, fileData []byte) (libSrv *library.Service, rommSrv *rommsrv.Service, cm *config.ConfigManager) {
	cm = config.NewConfigManager()
	cm.ConfigPath = filepath.Join(tempDir, "config.json")
	cm.Config = &types.AppConfig{LibraryPath: tempDir}

	rommSrv = rommsrv.New(mockRommConfig{})
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

	libSrv = library.New(cm, rommSrv, &MockUIProvider{})
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

	err = s.UploadSave(1, "../../etc", "passwd", "default")
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

	localPath := filepath.Join(tempDir, "snes", "1", "saves", "game.srm")
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

	err = s.UploadSave(1, "snes", "game.srm", "default")
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

func TestBridgeGameSaves_CrossCore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test_bridge")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	game := types.Game{ID: 10, PlatformSlug: "gba", FullPath: "gba/pokemon.gba"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)
	s := New(lib, romm, &MockUIProvider{})

	// Create save in mgba_libretro
	mgbaDir := filepath.Join(tempDir, "gba", "10", "saves", "mgba_libretro")
	if err := os.MkdirAll(mgbaDir, 0o755); err != nil {
		t.Fatalf("failed to create mgba dir: %v", err)
	}
	mgbaFile := filepath.Join(mgbaDir, "pokemon.srm")
	if err := os.WriteFile(mgbaFile, []byte("mgba_save_data"), 0o644); err != nil {
		t.Fatalf("failed to write mgba save: %v", err)
	}
	t1 := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	_ = os.Chtimes(mgbaFile, t1, t1)

	// Flatten / Bridge moves legacy core save to flat saves directory
	if err := s.BridgeGameSaves(10, ""); err != nil {
		t.Fatalf("BridgeGameSaves failed: %v", err)
	}

	flatFile := filepath.Join(tempDir, "gba", "10", "saves", "pokemon.srm")
	flatData, err := os.ReadFile(flatFile)
	if err != nil {
		t.Fatalf("failed to read migrated flat save: %v", err)
	}
	if string(flatData) != "mgba_save_data" {
		t.Errorf("expected migrated content 'mgba_save_data', got %q", string(flatData))
	}
	flatInfo, err := os.Stat(flatFile)
	if err != nil || !flatInfo.ModTime().Equal(t1) {
		t.Errorf("expected flat file to preserve modtime %v, got %v", t1, flatInfo.ModTime())
	}
	if _, err := os.Stat(mgbaFile); !os.IsNotExist(err) {
		t.Errorf("expected legacy mgba save to be removed after migration")
	}

	// Now simulate user having a newer save in another core folder
	gpspDir := filepath.Join(tempDir, "gba", "10", "saves", "gpsp_libretro")
	_ = os.MkdirAll(gpspDir, 0o755)
	gpspFile := filepath.Join(gpspDir, "pokemon.srm")
	t2 := time.Now().Add(-1 * time.Hour).Truncate(time.Second)
	if err := os.WriteFile(gpspFile, []byte("newer_gpsp_save"), 0o644); err != nil {
		t.Fatalf("failed to update gpsp save: %v", err)
	}
	_ = os.Chtimes(gpspFile, t2, t2)

	// Flatten again: newer gpsp save should replace flat save, and flat save backed up to .bak
	if err := s.BridgeGameSaves(10, ""); err != nil {
		t.Fatalf("BridgeGameSaves failed: %v", err)
	}

	flatData2, err := os.ReadFile(flatFile)
	if err != nil {
		t.Fatalf("failed to read flat save: %v", err)
	}
	if string(flatData2) != "newer_gpsp_save" {
		t.Errorf("expected flat save to be updated to 'newer_gpsp_save', got %q", string(flatData2))
	}

	bakData, err := os.ReadFile(flatFile + ".bak")
	if err != nil || string(bakData) != "mgba_save_data" {
		t.Errorf("expected backup to contain 'mgba_save_data', got %q (err: %v)", string(bakData), err)
	}
}

func TestBridgeGameSaves_SpecialCores(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test_special")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	game := types.Game{ID: 20, PlatformSlug: "ps2", FullPath: "ps2/game.iso"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)
	s := New(lib, romm, &MockUIProvider{})

	// Should not bridge or error for special platform / core
	if err := s.BridgeGameSaves(20, corePCSX2); err != nil {
		t.Errorf("expected nil error for special core, got %v", err)
	}
	if err := s.BridgeGameSaves(20, coreDolphin); err != nil {
		t.Errorf("expected nil error for dolphin, got %v", err)
	}
}

func TestDeleteGameFile_CrossCore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test_del_cross")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	game := types.Game{ID: 30, PlatformSlug: "gba", FullPath: "gba/game.gba"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)
	s := New(lib, romm, &MockUIProvider{})

	savesDir := filepath.Join(tempDir, "gba", "30", "saves")
	mgbaFile := filepath.Join(savesDir, "mgba_libretro", "game.srm")
	gpspFile := filepath.Join(savesDir, "gpsp_libretro", "game.srm")
	flatFile := filepath.Join(savesDir, "game.srm")

	_ = os.MkdirAll(filepath.Dir(mgbaFile), 0o755)
	_ = os.MkdirAll(filepath.Dir(gpspFile), 0o755)
	_ = os.WriteFile(mgbaFile, []byte("data"), 0o644)
	_ = os.WriteFile(gpspFile, []byte("data"), 0o644)
	_ = os.WriteFile(flatFile, []byte("data"), 0o644)

	if err := s.DeleteGameFile(30, constants.DirSaves, "mgba_libretro", "game.srm"); err != nil {
		t.Fatalf("DeleteGameFile failed: %v", err)
	}

	if _, err := os.Stat(mgbaFile); !os.IsNotExist(err) {
		t.Errorf("expected mgba file to be deleted")
	}
	if _, err := os.Stat(gpspFile); !os.IsNotExist(err) {
		t.Errorf("expected gpsp file to be deleted across cores")
	}
	if _, err := os.Stat(flatFile); !os.IsNotExist(err) {
		t.Errorf("expected flat file to be deleted across cores")
	}
}

func TestDeduplicateSaveItems(t *testing.T) {
	items := []types.FileItem{
		{Name: "game.srm", Core: "", UpdatedAt: "2026-10-07T10:00:00Z"},
		{Name: "game.srm", Core: "mgba_libretro", UpdatedAt: "2026-10-07T12:00:00Z"},
		{Name: "game.srm", Core: "gpsp_libretro", UpdatedAt: "2026-10-07T11:00:00Z"},
		{Name: "other.srm", Core: "snes9x_libretro", UpdatedAt: "2026-10-07T12:00:00Z"},
	}

	deduped := deduplicateSaveItems(items)
	if len(deduped) != 2 {
		t.Fatalf("expected 2 items, got %d", len(deduped))
	}
	if deduped[0].Name != "game.srm" || deduped[0].Core != "mgba_libretro" {
		t.Errorf("expected game.srm with mgba_libretro (newest), got %+v", deduped[0])
	}
	if deduped[1].Name != "other.srm" {
		t.Errorf("expected other.srm, got %+v", deduped[1])
	}
}

func TestBridgeGameSaves_BackupOnOverwrite(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test_bak")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	game := types.Game{ID: 40, PlatformSlug: "gba", FullPath: "gba/game.gba"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)
	s := New(lib, romm, &MockUIProvider{})

	savesDir := filepath.Join(tempDir, "gba", "40", "saves")
	flatFile := filepath.Join(savesDir, "game.srm")
	mgbaFile := filepath.Join(savesDir, "mgba_libretro", "game.srm")

	_ = os.MkdirAll(filepath.Dir(mgbaFile), 0o755)

	tOld := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	_ = os.WriteFile(flatFile, []byte("old_flat_progress"), 0o644)
	_ = os.Chtimes(flatFile, tOld, tOld)

	tNew := time.Now().Add(-1 * time.Hour).Truncate(time.Second)
	_ = os.WriteFile(mgbaFile, []byte("new_mgba_progress"), 0o644)
	_ = os.Chtimes(mgbaFile, tNew, tNew)

	if err := s.BridgeGameSaves(40, ""); err != nil {
		t.Fatalf("BridgeGameSaves failed: %v", err)
	}

	// Verify flatFile was updated with new_mgba_progress
	data, _ := os.ReadFile(flatFile)
	if string(data) != "new_mgba_progress" {
		t.Errorf("expected flat save to be updated to new_mgba_progress, got %q", string(data))
	}

	// Verify backup was created so old progress was not lost
	bakFile := flatFile + ".bak"
	bakData, err := os.ReadFile(bakFile)
	if err != nil {
		t.Fatalf("expected backup file %s to exist: %v", bakFile, err)
	}
	if string(bakData) != "old_flat_progress" {
		t.Errorf("expected backup content 'old_flat_progress', got %q", string(bakData))
	}
}

func TestBridgeGameSaves_BackfillPlatformCores(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test_backfill")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	game := types.Game{ID: 50, PlatformSlug: "gb", FullPath: "gb/zelda.gb"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)
	s := New(lib, romm, &MockUIProvider{})

	// Only 1 core folder exists initially
	savesDir := filepath.Join(tempDir, "gb", "50", "saves")
	gambatteFile := filepath.Join(savesDir, "gambatte_libretro", "zelda.srm")
	_ = os.MkdirAll(filepath.Dir(gambatteFile), 0o755)
	_ = os.WriteFile(gambatteFile, []byte("zelda_save"), 0o644)

	// Flatten / migrate legacy core save
	if err := s.BridgeGameSaves(50, ""); err != nil {
		t.Fatalf("BridgeGameSaves failed: %v", err)
	}

	// Check flat save was created
	flatFile := filepath.Join(savesDir, "zelda.srm")
	if data, err := os.ReadFile(flatFile); err != nil || string(data) != "zelda_save" {
		t.Errorf("expected flat save file with 'zelda_save', got %q (err: %v)", string(data), err)
	}

	// Legacy folder should be removed
	if _, err := os.Stat(gambatteFile); !os.IsNotExist(err) {
		t.Errorf("expected legacy gambatte save to be cleaned up")
	}
}

func TestBridgeGameSaves_RomFilesNeverCopied(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test_rom_safety")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	game := types.Game{ID: 60, PlatformSlug: "gba", FullPath: "gba/metroid.gba"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)
	s := New(lib, romm, &MockUIProvider{})

	romDir := filepath.Join(tempDir, "gba", "60")
	_ = os.MkdirAll(romDir, 0o755)
	romZip := filepath.Join(romDir, "metroid.zip")
	romGba := filepath.Join(romDir, "metroid.gba")
	_ = os.WriteFile(romZip, []byte("rom_zip_data"), 0o644)
	_ = os.WriteFile(romGba, []byte("rom_gba_data"), 0o644)

	// Save is in saves/mgba_libretro/
	savesDir := filepath.Join(romDir, "saves")
	mgbaFile := filepath.Join(savesDir, "mgba_libretro", "metroid.srm")
	_ = os.MkdirAll(filepath.Dir(mgbaFile), 0o755)
	_ = os.WriteFile(mgbaFile, []byte("save_data"), 0o644)

	if err := s.BridgeGameSaves(60, ""); err != nil {
		t.Fatalf("BridgeGameSaves failed: %v", err)
	}

	// Verify metroid.srm was migrated to flat savesDir
	flatFile := filepath.Join(savesDir, "metroid.srm")
	if data, err := os.ReadFile(flatFile); err != nil || string(data) != "save_data" {
		t.Errorf("expected metroid.srm to be in flat saves directory")
	}

	// Verify ROM files are NEVER copied to saves directory or core subdirectories
	copiedZip := filepath.Join(savesDir, "metroid.zip")
	if _, err := os.Stat(copiedZip); !os.IsNotExist(err) {
		t.Errorf("ROM zip file was unexpectedly copied to saves directory")
	}
	copiedGba := filepath.Join(savesDir, "vba_next_libretro", "metroid.gba")
	if _, err := os.Stat(copiedGba); !os.IsNotExist(err) {
		t.Errorf("ROM gba file was unexpectedly copied to saves directory")
	}
}

func TestDownloadServerSave_BackupOnOverwrite(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test_dl_bak")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	game := types.Game{ID: 70, PlatformSlug: "gba", FullPath: "gba/game.gba"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, []byte("server_save_content"))
	s := New(lib, romm, &MockUIProvider{})

	localSave := filepath.Join(tempDir, "gba", "70", "saves", "game.srm")
	_ = os.MkdirAll(filepath.Dir(localSave), 0o755)
	_ = os.WriteFile(localSave, []byte("local_progress_to_preserve"), 0o644)

	if err := s.DownloadServerSave(70, 100, "mgba_libretro", "game.srm", ""); err != nil {
		t.Fatalf("DownloadServerSave failed: %v", err)
	}

	// Verify local save was updated with server content
	data, _ := os.ReadFile(localSave)
	if string(data) != "server_save_content" {
		t.Errorf("expected local save to have server content, got %q", string(data))
	}

	// Verify backup of previous local progress was created
	bakFile := localSave + ".bak"
	bakData, err := os.ReadFile(bakFile)
	if err != nil {
		t.Fatalf("expected .bak backup to exist: %v", err)
	}
	if string(bakData) != "local_progress_to_preserve" {
		t.Errorf("expected backup to have 'local_progress_to_preserve', got %q", string(bakData))
	}
}

func TestBridgeGameSaves_OlderCoreSavePreservedAsBackup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test_older_core")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	game := types.Game{ID: 80, PlatformSlug: "gba", FullPath: "gba/game.gba"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)
	s := New(lib, romm, &MockUIProvider{})

	savesDir := filepath.Join(tempDir, "gba", "80", "saves")
	flatFile := filepath.Join(savesDir, "game.srm")
	coreDir := filepath.Join(savesDir, "mgba_libretro")
	coreFile := filepath.Join(coreDir, "game.srm")

	_ = os.MkdirAll(coreDir, 0o755)

	tNew := time.Now().Add(-1 * time.Hour).Truncate(time.Second)
	_ = os.WriteFile(flatFile, []byte("newer_flat_save"), 0o644)
	_ = os.Chtimes(flatFile, tNew, tNew)

	tOld := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	_ = os.WriteFile(coreFile, []byte("older_core_save_to_keep"), 0o644)
	_ = os.Chtimes(coreFile, tOld, tOld)

	if err := s.BridgeGameSaves(80, ""); err != nil {
		t.Fatalf("BridgeGameSaves failed: %v", err)
	}

	// Flat save should keep the newer content
	flatData, err := os.ReadFile(flatFile)
	if err != nil || string(flatData) != "newer_flat_save" {
		t.Errorf("expected flat save to keep 'newer_flat_save', got %q (err: %v)", string(flatData), err)
	}

	// Older core save should be preserved in .bak so nothing is lost
	bakFile := flatFile + ".bak"
	bakData, err := os.ReadFile(bakFile)
	if err != nil {
		t.Fatalf("expected backup file %s to exist: %v", bakFile, err)
	}
	if string(bakData) != "older_core_save_to_keep" {
		t.Errorf("expected backup to have 'older_core_save_to_keep', got %q", string(bakData))
	}

	// Legacy core folder should be cleaned up
	if _, err := os.Stat(coreFile); !os.IsNotExist(err) {
		t.Errorf("expected legacy core file to be removed after safe migration")
	}
}

func TestBridgeGameSaves_IdenticalFilesDeduplicated(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test_identical")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	game := types.Game{ID: 90, PlatformSlug: "gba", FullPath: "gba/game.gba"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)
	s := New(lib, romm, &MockUIProvider{})

	savesDir := filepath.Join(tempDir, "gba", "90", "saves")
	flatFile := filepath.Join(savesDir, "game.srm")
	coreDir := filepath.Join(savesDir, "mgba_libretro")
	coreFile := filepath.Join(coreDir, "game.srm")

	_ = os.MkdirAll(coreDir, 0o755)
	_ = os.WriteFile(flatFile, []byte("identical_content"), 0o644)
	_ = os.WriteFile(coreFile, []byte("identical_content"), 0o644)

	if err := s.BridgeGameSaves(90, ""); err != nil {
		t.Fatalf("BridgeGameSaves failed: %v", err)
	}

	// Flat save remains
	flatData, err := os.ReadFile(flatFile)
	if err != nil || string(flatData) != "identical_content" {
		t.Errorf("expected flat save to have 'identical_content', got %q", string(flatData))
	}

	// Core file removed and no unnecessary .bak created
	if _, err := os.Stat(coreFile); !os.IsNotExist(err) {
		t.Errorf("expected core file to be removed")
	}
	if _, err := os.Stat(flatFile + ".bak"); !os.IsNotExist(err) {
		t.Errorf("did not expect .bak file for identical contents")
	}
}

func TestBridgeGameSaves_OrphanBackupMigrated(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test_orphan_bak")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	game := types.Game{ID: 95, PlatformSlug: "gba", FullPath: "gba/game.gba"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)
	s := New(lib, romm, &MockUIProvider{})

	savesDir := filepath.Join(tempDir, "gba", "95", "saves")
	coreDir := filepath.Join(savesDir, "mgba_libretro")
	orphanBak := filepath.Join(coreDir, "game.srm.bak")

	_ = os.MkdirAll(coreDir, 0o755)
	_ = os.WriteFile(orphanBak, []byte("orphan_backup_data"), 0o644)

	if err := s.BridgeGameSaves(95, ""); err != nil {
		t.Fatalf("BridgeGameSaves failed: %v", err)
	}

	// Flat backup should exist
	flatBak := filepath.Join(savesDir, "game.srm.bak")
	data, err := os.ReadFile(flatBak)
	if err != nil || string(data) != "orphan_backup_data" {
		t.Errorf("expected flat backup with 'orphan_backup_data', got %q (err: %v)", string(data), err)
	}

	// Core dir should be cleaned up
	if _, err := os.Stat(orphanBak); !os.IsNotExist(err) {
		t.Errorf("expected orphan backup to be moved from core dir")
	}
}

func TestGetSaves_GameCube_Variants(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test_gc_variants")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	game := types.Game{ID: 10, PlatformSlug: "gamecube", FullPath: "gamecube/game.iso"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)
	s := New(lib, romm, &MockUIProvider{})

	romDir := s.library.GetRomDir(&game)
	savesDir := filepath.Join(romDir, constants.DirSaves)
	gcBase := filepath.Join(savesDir, "dolphin-emu", "User", "GC")
	filesToCreate := []struct {
		relDir   string
		filename string
	}{
		{"USA/Card A", "GM8E01.gci"},
		{"EUR/Card B", "GM8P01.gci"},
		{"JAP/Card A", "GM8J01.gci"},
		{"JPN/Card A", "JPN01.gci"},
		{"", "MemoryCardA.USA.raw"},
		{"", "MemoryCardB.EUR.gcp"},
	}

	for _, fc := range filesToCreate {
		dir := filepath.Join(gcBase, fc.relDir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("failed to mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, fc.filename), []byte("save data"), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", fc.filename, err)
		}
	}
	// Write SRAM.raw in User/GC/ to ensure it is ignored
	if err := os.WriteFile(filepath.Join(gcBase, "SRAM.raw"), []byte("sram_data"), 0o644); err != nil {
		t.Fatalf("failed to write SRAM.raw: %v", err)
	}

	saves, err := s.GetSaves(10)
	if err != nil {
		t.Fatalf("GetSaves failed: %v", err)
	}

	if len(saves) != len(filesToCreate) {
		t.Fatalf("expected %d saves, got %d: %+v", len(filesToCreate), len(saves), saves)
	}

	saveMap := make(map[string]string)
	for _, sv := range saves {
		saveMap[sv.Name] = sv.Core
	}

	if _, ok := saveMap["SRAM.raw"]; ok {
		t.Errorf("SRAM.raw should not be tracked as a save file")
	}

	expected := map[string]string{
		"GM8E01.gci":          "dolphin-emu/User/GC/USA/Card A",
		"GM8P01.gci":          "dolphin-emu/User/GC/EUR/Card B",
		"GM8J01.gci":          "dolphin-emu/User/GC/JAP/Card A",
		"JPN01.gci":           "dolphin-emu/User/GC/JPN/Card A",
		"MemoryCardA.USA.raw": "dolphin-emu/User/GC",
		"MemoryCardB.EUR.gcp": "dolphin-emu/User/GC",
	}

	for name, expCore := range expected {
		if core, ok := saveMap[name]; !ok {
			t.Errorf("missing save %s", name)
		} else if core != expCore {
			t.Errorf("for save %s, expected core %q, got %q", name, expCore, core)
		}
	}
}

func TestDownloadServerSave_GameCube(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test_gc_dl")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fakeData := []byte("gc_server_save")
	game := types.Game{ID: 10, PlatformSlug: "gamecube", FullPath: "gamecube/game.iso"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, fakeData)
	s := New(lib, romm, &MockUIProvider{})

	tests := []struct {
		name         string
		core         string
		filename     string
		expectedPath string
	}{
		{
			name:         "Card A emulator name with USA GCI",
			core:         "Card A",
			filename:     "GM8E01.gci",
			expectedPath: filepath.Join("dolphin-emu", "User", "GC", "USA", "Card A", "GM8E01.gci"),
		},
		{
			name:         "Card B emulator name with EUR GCI",
			core:         "Card B",
			filename:     "GM8P01.gci",
			expectedPath: filepath.Join("dolphin-emu", "User", "GC", "EUR", "Card B", "GM8P01.gci"),
		},
		{
			name:         "dolphin_libretro core detects JAP region from GCI",
			core:         "dolphin_libretro",
			filename:     "GM8J01.gci",
			expectedPath: filepath.Join("dolphin-emu", "User", "GC", "JAP", "Card A", "GM8J01.gci"),
		},
		{
			name:         "Full path core",
			core:         "dolphin-emu/User/GC/USA/Card A",
			filename:     "CustomUSA.gci",
			expectedPath: filepath.Join("dolphin-emu", "User", "GC", "USA", "Card A", "CustomUSA.gci"),
		},
		{
			name:         "Windows backslash core",
			core:         `dolphin-emu\User\GC\USA\Card A`,
			filename:     "WinUSA.gci",
			expectedPath: filepath.Join("dolphin-emu", "User", "GC", "USA", "Card A", "WinUSA.gci"),
		},
		{
			name:         "Raw memory card",
			core:         "dolphin_libretro",
			filename:     "MemoryCardA.USA.raw",
			expectedPath: filepath.Join("dolphin-emu", "User", "GC", "MemoryCardA.USA.raw"),
		},
	}

	romDir := s.library.GetRomDir(&game)
	savesBase := filepath.Join(romDir, constants.DirSaves)
	for idx, tc := range tests {
		err := s.DownloadServerSave(10, uint(100+idx), tc.core, tc.filename, "")
		if err != nil {
			t.Errorf("test %q failed: %v", tc.name, err)
			continue
		}
		fullExpected := filepath.Join(savesBase, tc.expectedPath)
		if _, err := os.Stat(fullExpected); os.IsNotExist(err) {
			t.Errorf("test %q: expected file at %s but not found", tc.name, fullExpected)
		}
	}
}

func TestDeleteGameFile_GameCube(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test_gc_del")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	game := types.Game{ID: 10, PlatformSlug: "gamecube", FullPath: "gamecube/game.iso"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)
	s := New(lib, romm, &MockUIProvider{})

	romDir := s.library.GetRomDir(&game)
	savesDir := filepath.Join(romDir, constants.DirSaves)
	gcDir := filepath.Join(savesDir, "dolphin-emu", "User", "GC", "USA", "Card A")
	_ = os.MkdirAll(gcDir, 0o755)

	targetFile1 := filepath.Join(gcDir, "GM8E01.gci")
	_ = os.WriteFile(targetFile1, []byte("data1"), 0o644)

	// Delete using "Card A" as core
	if err := s.DeleteGameFile(10, "saves", "Card A", "GM8E01.gci"); err != nil {
		t.Fatalf("DeleteGameFile Card A failed: %v", err)
	}
	if _, err := os.Stat(targetFile1); !os.IsNotExist(err) {
		t.Errorf("expected %s to be deleted", targetFile1)
	}

	targetFile2 := filepath.Join(gcDir, "WinUSA.gci")
	_ = os.WriteFile(targetFile2, []byte("data2"), 0o644)

	// Delete using Windows backslash core
	if err := s.DeleteGameFile(10, "saves", `dolphin-emu\User\GC\USA\Card A`, "WinUSA.gci"); err != nil {
		t.Fatalf("DeleteGameFile Windows path failed: %v", err)
	}
	if _, err := os.Stat(targetFile2); !os.IsNotExist(err) {
		t.Errorf("expected %s to be deleted", targetFile2)
	}

	rawFile := filepath.Join(savesDir, "dolphin-emu", "User", "GC", "MemoryCardA.USA.raw")
	_ = os.WriteFile(rawFile, []byte("raw_data"), 0o644)

	// Delete raw card using dolphin-emu/User/GC
	if err := s.DeleteGameFile(10, "saves", "dolphin-emu/User/GC", "MemoryCardA.USA.raw"); err != nil {
		t.Fatalf("DeleteGameFile raw card failed: %v", err)
	}
	if _, err := os.Stat(rawFile); !os.IsNotExist(err) {
		t.Errorf("expected %s to be deleted", rawFile)
	}
}

func TestGameCube_AutoMigrateMisplaced(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test_gc_migrate")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	game := types.Game{ID: 10, PlatformSlug: "gamecube", FullPath: "gamecube/game.iso"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)
	s := New(lib, romm, &MockUIProvider{})

	romDir := s.library.GetRomDir(&game)
	savesDir := filepath.Join(romDir, constants.DirSaves)
	_ = os.MkdirAll(savesDir, 0o755)

	misplacedGCI := filepath.Join(savesDir, "GM8E01.gci")
	misplacedRaw := filepath.Join(savesDir, "MemoryCardA.USA.raw")
	_ = os.WriteFile(misplacedGCI, []byte("gci_data"), 0o644)
	_ = os.WriteFile(misplacedRaw, []byte("raw_data"), 0o644)

	if err := s.FlattenGameSaves(10); err != nil {
		t.Fatalf("FlattenGameSaves failed: %v", err)
	}

	// GM8E01.gci should be moved to dolphin-emu/User/GC/USA/Card A/
	expectedGCI := filepath.Join(savesDir, "dolphin-emu", "User", "GC", "USA", "Card A", "GM8E01.gci")
	if _, err := os.Stat(expectedGCI); os.IsNotExist(err) {
		t.Errorf("expected misplaced GCI to be migrated to %s", expectedGCI)
	}
	if _, err := os.Stat(misplacedGCI); !os.IsNotExist(err) {
		t.Errorf("expected original misplaced GCI to be removed")
	}

	// MemoryCardA.USA.raw should be moved to dolphin-emu/User/GC/
	expectedRaw := filepath.Join(savesDir, "dolphin-emu", "User", "GC", "MemoryCardA.USA.raw")
	if _, err := os.Stat(expectedRaw); os.IsNotExist(err) {
		t.Errorf("expected misplaced raw card to be migrated to %s", expectedRaw)
	}
	if _, err := os.Stat(misplacedRaw); !os.IsNotExist(err) {
		t.Errorf("expected original misplaced raw card to be removed")
	}
}

func TestGetSaves_GameCube_MigratesUserDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test_gc_user_migrate")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	game := types.Game{ID: 10, PlatformSlug: "gamecube", FullPath: "gamecube/game.iso"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)
	s := New(lib, romm, &MockUIProvider{})

	romDir := s.library.GetRomDir(&game)
	savesDir := filepath.Join(romDir, constants.DirSaves)
	userDir := filepath.Join(savesDir, "User", "GC", "USA", "Card A")
	_ = os.MkdirAll(userDir, 0o755)
	_ = os.WriteFile(filepath.Join(userDir, "01-GKYE-test.gci"), []byte("gci_data"), 0o644)

	saves, err := s.GetSaves(10)
	if err != nil {
		t.Fatalf("GetSaves failed: %v", err)
	}

	if len(saves) != 1 || saves[0].Name != "01-GKYE-test.gci" {
		t.Fatalf("expected 1 save with name 01-GKYE-test.gci, got: %+v", saves)
	}

	// Verify User directory was migrated to dolphin-emu/User
	migratedFile := filepath.Join(savesDir, "dolphin-emu", "User", "GC", "USA", "Card A", "01-GKYE-test.gci")
	if _, err := os.Stat(migratedFile); os.IsNotExist(err) {
		t.Errorf("expected file to be migrated to %s", migratedFile)
	}
	if _, err := os.Stat(filepath.Join(savesDir, "User")); !os.IsNotExist(err) {
		t.Errorf("expected saves/User to be removed after migration")
	}
}

func TestGetStates_GameCube_Dolphin(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sync_test_gc_states")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	game := types.Game{ID: 10, PlatformSlug: "gamecube", FullPath: "gamecube/game.iso"}
	gameData, _ := json.Marshal(game)

	lib, romm, _ := setupServices(tempDir, gameData, nil)
	s := New(lib, romm, &MockUIProvider{})

	romDir := s.library.GetRomDir(&game)
	statesDir := filepath.Join(romDir, constants.DirStates, "dolphin-emu")
	_ = os.MkdirAll(statesDir, 0o755)
	_ = os.WriteFile(filepath.Join(statesDir, "game.state"), []byte("state_data"), 0o644)

	states, err := s.GetStates(10)
	if err != nil {
		t.Fatalf("GetStates failed: %v", err)
	}

	if len(states) != 1 || states[0].Name != "game.state" || states[0].Core != "dolphin-emu" {
		t.Fatalf("expected 1 state with name game.state and core dolphin-emu, got: %+v", states)
	}
}
