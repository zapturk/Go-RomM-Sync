package main

import (
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
	cm := &config.ConfigManager{
		ConfigPath: configPath,
		Config: &types.AppConfig{
			RommHost: "http://initial.com",
			Username: "initial-user",
		},
	}

	app := NewApp(cm)

	// 1. Test partial update
	update := types.AppConfig{
		Username: "new-user",
		// RommHost is empty, should be preserved
	}

	res := app.SaveConfig(update)
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

func TestRommClientLifecycle(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "app-lifecycle-test")
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	cm := &config.ConfigManager{
		ConfigPath: configPath,
		Config: &types.AppConfig{
			RommHost: "http://host1.com",
		},
	}

	app := NewApp(cm)
	initialClient := app.rommClient

	// 1. Save with same host
	app.SaveConfig(types.AppConfig{Username: "user1"})
	if app.rommClient != initialClient {
		t.Error("RomM client should NOT have been recreated when host remained the same")
	}

	// 2. Save with different host
	app.SaveConfig(types.AppConfig{RommHost: "http://host2.com"})
	if app.rommClient == initialClient {
		t.Error("RomM client SHOULD have been recreated when host changed")
	}
	if app.rommClient.BaseURL != "http://host2.com" {
		t.Errorf("Expected client URL http://host2.com, got %s", app.rommClient.BaseURL)
	}
}
func TestPathTraversalValidation(t *testing.T) {
	app := &App{}

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
