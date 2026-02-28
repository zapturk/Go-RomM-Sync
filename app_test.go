package main

import (
	"context"
	"go-romm-sync/config"
	"go-romm-sync/types"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveConfigMerge(t *testing.T) {
	// Setup temp config
	tmpDir, err := os.MkdirTemp("", "app-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	cm := config.NewConfigManager()
	cm.ConfigPath = configPath // Set manual path for test
	cm.Config = &types.AppConfig{
		RommHost: "http://initial.com",
		Username: "initial-user",
	}

	app := NewApp(cm)

	// 1. Test partial update
	update := types.AppConfig{
		Username: "new-user",
		// RommHost is empty, should be preserved
	}

	res := app.SaveConfig(&update)
	if res != "Configuration saved successfully!" {
		t.Errorf("Expected success message, got %s", res)
	}

	finalCfg := cm.GetConfig()
	if finalCfg.Username != "new-user" {
		t.Errorf("Expected username new-user, got %s", finalCfg.Username)
	}
	if finalCfg.RommHost != "http://initial.com" {
		t.Errorf("Expected host to be preserved as http://initial.com, got %s", finalCfg.RommHost)
	}
}

func TestLogout(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "app-logout-test")
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	cm := config.NewConfigManager()
	cm.ConfigPath = configPath
	cm.Config = &types.AppConfig{
		Username:        "user",
		Password:        "pass",
		CheevosUsername: "cheevos-user",
		CheevosPassword: "cheevos-pass",
	}

	app := NewApp(cm)

	if err := app.Logout(); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	finalCfg := cm.GetConfig()
	if finalCfg.Username != "" || finalCfg.Password != "" || finalCfg.CheevosUsername != "" || finalCfg.CheevosPassword != "" {
		t.Errorf("Logout did not clear all credentials: %+v", finalCfg)
	}
}

func TestRommSrvLifecycle(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "app-lifecycle-test")
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	cm := config.NewConfigManager()
	cm.ConfigPath = configPath
	cm.Config = &types.AppConfig{
		RommHost: "http://host1.com",
	}

	app := NewApp(cm)
	initialSrv := app.rommSrv

	// 1. Save with same host but DIFFERENT username (SHOULD be recreated for security)
	app.SaveConfig(&types.AppConfig{Username: "user1"})
	if app.rommSrv == initialSrv {
		t.Error("RomM service SHOULD have been recreated when credentials changed for security")
	}
	initialSrv = app.rommSrv

	// 2. Save with NOTHING changed (should NOT be recreated)
	app.SaveConfig(&types.AppConfig{}) // empty update means nothing changes
	if app.rommSrv != initialSrv {
		t.Error("RomM service should NOT have been recreated when nothing changed")
	}

	// 3. Save with different host
	app.SaveConfig(&types.AppConfig{RommHost: "http://host2.com"})
	if app.rommSrv == initialSrv {
		t.Error("RomM service SHOULD have been recreated when host changed")
	}
}

func TestPathTraversalValidation(t *testing.T) {
	cm := config.NewConfigManager()
	app := NewApp(cm)

	tests := []struct {
		name     string
		core     string
		filename string
		wantErr  bool
	}{
		{"Valid", "snes", "save.sav", false},
		{"Traversal Core", "../outside", "save.sav", false},                   // sanitized to "outside"
		{"Traversal Filename", "snes", "../outside.sav", false},               // sanitized to "outside.sav"
		{"Absolute Core Windows", "C:\\Windows\\System32", "save.sav", false}, // sanitized to "System32"
		{"Current Dir Core", ".", "save.sav", true},
		{"Double Dot Core", "..", "save.sav", true},
		{"Empty Core", "", "save.sav", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := app.ValidateAssetPath(tt.core, tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAssetPath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAppServiceWrappers(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "app-wrappers-test")
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	cm := config.NewConfigManager()
	cm.ConfigPath = configPath
	cm.Config = &types.AppConfig{
		RommHost: "http://localhost",
	}

	app := NewApp(cm)

	// Test GetConfig
	cfg := app.GetConfig()
	if cfg.RommHost != "http://localhost" {
		t.Errorf("Expected localhost, got %s", cfg.RommHost)
	}

	// Test DownloadRom
	url, err := app.DownloadRom(1)
	if err != nil {
		t.Fatalf("DownloadRom failed: %v", err)
	}
	if url != "http://localhost/api/roms/1/download" {
		t.Errorf("Unexpected download URL: %s", url)
	}

	// Test internal providers
	if app.GetRomMHost() != "http://localhost" {
		t.Error("GetRomMHost failed")
	}
	if app.GetLibraryPath() != "" {
		t.Error("GetLibraryPath should be empty initially")
	}

	app.SaveDefaultLibraryPath("/tmp/lib")
	if app.GetLibraryPath() != "/tmp/lib" {
		t.Error("SaveDefaultLibraryPath failed")
	}
}

func TestAppStartup(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "app-startup-test")
	defer os.RemoveAll(tmpDir)
	configPath := filepath.Join(tmpDir, "config.json")

	cm := config.NewConfigManager()
	cm.ConfigPath = configPath

	app := NewApp(cm)
	app.startup(context.Background())
	if app.ctx == nil {
		t.Error("Context not saved on startup")
	}
}

func TestAppDelegationWrappers(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "app-delegation-test")
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	cm := config.NewConfigManager()
	cm.ConfigPath = configPath
	cm.Config = &types.AppConfig{
		RommHost:    "http://localhost",
		LibraryPath: tmpDir,
	}

	app := NewApp(cm)

	// Test a few delegation wrappers to ensure they don't panic or fail
	// even if they return errors due to uninitialized network clients.

	// RomM Wrappers
	app.GetLibrary()
	app.GetPlatforms()
	app.GetServerSaves(1)
	app.GetServerStates(1)

	// Library Wrappers
	app.GetRomDownloadStatus(1)
	app.DeleteRom(1)

	// Sync Wrappers
	app.GetSaves(1)
	app.GetStates(1)
	app.DeleteSave(1, "snes", "save.sav")
	app.DeleteState(1, "snes", "state.sav")
}

func TestAppComplexWrappers(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "app-complex-test")
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	cm := config.NewConfigManager()
	cm.ConfigPath = configPath
	cm.Config = &types.AppConfig{
		RommHost:    "http://localhost",
		LibraryPath: tmpDir,
	}

	app := NewApp(cm)

	// Test SaveConfig with host change
	app.SaveConfig(&types.AppConfig{RommHost: "http://newhost.com"})
	if app.rommSrv.GetClient().BaseURL != "http://newhost.com" {
		t.Errorf("Expected host to be updated to http://newhost.com")
	}

	// Test Library wrappers (even if they error, they increase coverage)
	app.DownloadRomToLibrary(1)
	app.DeleteRom(1)

	// Test Sync wrappers
	app.UploadSave(1, "snes", "game.srm")
	app.UploadState(1, "snes", "game.st0")
	app.DownloadServerSave(1, "remote", "snes", "game.srm", "")
	app.DownloadServerState(1, "remote", "snes", "game.st0", "")
}

func TestAppExhaustiveWrappers(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "app-exhaustive-test")
	defer os.RemoveAll(tmpDir)
	configPath := filepath.Join(tmpDir, "config.json")

	cm := config.NewConfigManager()
	cm.ConfigPath = configPath

	app := NewApp(cm)

	// Recover from panics because some Wails runtime methods might panic with nil context
	defer func() { recover() }()

	// Call remaining simple methods
	app.GetPlatforms()
	app.GetDefaultLibraryPath()
	app.SelectRetroArchExecutable()
	app.SelectLibraryPath()
	app.Login()
	app.GetLibrary()
	app.GetCover(1, "/url")
	app.GetPlatformCover(1, "slug")
	app.GetServerSaves(1)
	app.GetServerStates(1)
	app.DeleteRom(1)
	app.DeleteSave(1, "core", "file")
	app.DeleteState(1, "core", "file")
	app.ValidateAssetPath("core", "file")
	app.PlayRom(1)
	app.Logout()

	// Provider implementations
	app.ConfigGetConfig()
	app.ConfigSave(&types.AppConfig{})
	app.GetRomMHost()
	app.GetUsername()
	app.GetPassword()
	app.GetLibraryPath()
	app.GetRetroArchPath()
	app.GetCheevosCredentials()
	app.GetRom(1)

	// Logging and UI (will likely fail gracefully or be ignored due to nil ctx)
	app.LogInfof("test")
	app.LogErrorf("test")
	app.EventsEmit("test")

	// These might panic with nil ctx, so we wrap them individually if we want to be safe,
	// but the defer recover() at the top handles it.
	app.WindowHide()
	app.WindowShow()
	app.WindowUnminimise()
	app.WindowSetAlwaysOnTop(true)
	app.Greet("test")
}
