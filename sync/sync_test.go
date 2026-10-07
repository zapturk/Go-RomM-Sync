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

	// Bridge to target core gpsp_libretro
	if err := s.BridgeGameSaves(10, "gpsp_libretro"); err != nil {
		t.Fatalf("BridgeGameSaves failed: %v", err)
	}

	gpspFile := filepath.Join(tempDir, "gba", "10", "saves", "gpsp_libretro", "pokemon.srm")
	gpspData, err := os.ReadFile(gpspFile)
	if err != nil {
		t.Fatalf("failed to read bridged gpsp save: %v", err)
	}
	if string(gpspData) != "mgba_save_data" {
		t.Errorf("expected bridged content 'mgba_save_data', got %q", string(gpspData))
	}
	gpspInfo, err := os.Stat(gpspFile)
	if err != nil || !gpspInfo.ModTime().Equal(t1) {
		t.Errorf("expected bridged file to preserve modtime %v, got %v", t1, gpspInfo.ModTime())
	}

	// Now simulate user playing on gpsp and updating the save
	t2 := time.Now().Add(-1 * time.Hour).Truncate(time.Second)
	if err := os.WriteFile(gpspFile, []byte("newer_gpsp_save"), 0o644); err != nil {
		t.Fatalf("failed to update gpsp save: %v", err)
	}
	_ = os.Chtimes(gpspFile, t2, t2)

	// Bridge across all cores (empty targetCore)
	if err := s.BridgeGameSaves(10, ""); err != nil {
		t.Fatalf("BridgeGameSaves across all cores failed: %v", err)
	}

	mgbaData, err := os.ReadFile(mgbaFile)
	if err != nil {
		t.Fatalf("failed to read mgba save: %v", err)
	}
	if string(mgbaData) != "newer_gpsp_save" {
		t.Errorf("expected mgba save to be updated to 'newer_gpsp_save', got %q", string(mgbaData))
	}
	mgbaInfo, err := os.Stat(mgbaFile)
	if err != nil || !mgbaInfo.ModTime().Equal(t2) {
		t.Errorf("expected mgba file modtime to be updated to %v, got %v", t2, mgbaInfo.ModTime())
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
	mgbaFile := filepath.Join(savesDir, "mgba_libretro", "game.srm")
	gpspFile := filepath.Join(savesDir, "gpsp_libretro", "game.srm")

	_ = os.MkdirAll(filepath.Dir(mgbaFile), 0o755)
	_ = os.MkdirAll(filepath.Dir(gpspFile), 0o755)

	tOld := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	_ = os.WriteFile(gpspFile, []byte("old_gpsp_progress"), 0o644)
	_ = os.Chtimes(gpspFile, tOld, tOld)

	tNew := time.Now().Add(-1 * time.Hour).Truncate(time.Second)
	_ = os.WriteFile(mgbaFile, []byte("new_mgba_progress"), 0o644)
	_ = os.Chtimes(mgbaFile, tNew, tNew)

	// Bridge to target core gpsp
	if err := s.BridgeGameSaves(40, "gpsp_libretro"); err != nil {
		t.Fatalf("BridgeGameSaves failed: %v", err)
	}

	// Verify gpspFile was updated
	data, _ := os.ReadFile(gpspFile)
	if string(data) != "new_mgba_progress" {
		t.Errorf("expected gpsp save to be updated to new_mgba_progress, got %q", string(data))
	}

	// Verify backup was created so old progress was not lost
	bakFile := gpspFile + ".bak"
	bakData, err := os.ReadFile(bakFile)
	if err != nil {
		t.Fatalf("expected backup file %s to exist: %v", bakFile, err)
	}
	if string(bakData) != "old_gpsp_progress" {
		t.Errorf("expected backup content 'old_gpsp_progress', got %q", string(bakData))
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

	// Bridge across all cores (backfilling)
	if err := s.BridgeGameSaves(50, ""); err != nil {
		t.Fatalf("BridgeGameSaves failed: %v", err)
	}

	// Check that other GB platform cores (mgba_libretro, sameboy_libretro) got backfilled
	for _, core := range []string{"mgba_libretro", "sameboy_libretro"} {
		destFile := filepath.Join(savesDir, core, "zelda.srm")
		data, err := os.ReadFile(destFile)
		if err != nil {
			t.Errorf("expected backfilled file for core %s: %v", core, err)
			continue
		}
		if string(data) != "zelda_save" {
			t.Errorf("expected 'zelda_save' for core %s, got %q", core, string(data))
		}
	}

	// Check flat save was also backfilled
	flatFile := filepath.Join(savesDir, "zelda.srm")
	if data, err := os.ReadFile(flatFile); err != nil || string(data) != "zelda_save" {
		t.Errorf("expected flat save file to be backfilled")
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

	// Verify metroid.srm was bridged to vba_next_libretro
	vbaFile := filepath.Join(savesDir, "vba_next_libretro", "metroid.srm")
	if data, err := os.ReadFile(vbaFile); err != nil || string(data) != "save_data" {
		t.Errorf("expected metroid.srm to be bridged to vba_next_libretro")
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

	localSave := filepath.Join(tempDir, "gba", "70", "saves", "mgba_libretro", "game.srm")
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
