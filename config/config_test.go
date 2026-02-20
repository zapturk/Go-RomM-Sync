package config

import (
	"go-romm-sync/types"
	"os"
	"path/filepath"
	"testing"
)

func TestNewConfigManager(t *testing.T) {
	cm := NewConfigManager()
	if cm.ConfigPath == "" {
		t.Error("Expected ConfigPath to be set")
	}
	if cm.Config == nil {
		t.Error("Expected Config to be initialized")
	}
}

func TestLoadAndSave(t *testing.T) {
	// Create a temporary directory for the test config
	tmpDir, err := os.MkdirTemp("", "config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	cm := &ConfigManager{
		ConfigPath: configPath,
		Config:     &types.AppConfig{},
	}

	// 1. Test saving
	testConfig := types.AppConfig{
		RommHost:    "http://test.com",
		Username:    "user",
		LibraryPath: "/path/to/lib",
	}

	err = cm.Save(testConfig)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// 2. Test loading
	cm2 := &ConfigManager{
		ConfigPath: configPath,
		Config:     &types.AppConfig{},
	}
	err = cm2.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cm2.Config.RommHost != testConfig.RommHost {
		t.Errorf("Expected host %s, got %s", testConfig.RommHost, cm2.Config.RommHost)
	}
	if cm2.Config.Username != testConfig.Username {
		t.Errorf("Expected username %s, got %s", testConfig.Username, cm2.Config.Username)
	}
}

func TestCreateDefault(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-default-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "subdir", "config.json")
	cm := &ConfigManager{
		ConfigPath: configPath,
		Config:     &types.AppConfig{},
	}

	err = cm.Load()
	if err != nil {
		t.Fatalf("Load should not fail when file is missing (it should create default): %v", err)
	}

	if cm.Config.LibraryPath == "" {
		t.Error("Expected default LibraryPath to be set")
	}

	// Verify file was actually written to disk
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Default config file was not written to disk")
	}
}

func TestGetConfigThreadSafety(t *testing.T) {
	cm := &ConfigManager{
		Config: &types.AppConfig{RommHost: "initial"},
	}

	// Simple check that it returns a copy
	cfg := cm.GetConfig()
	cfg.RommHost = "modified"

	if cm.Config.RommHost != "initial" {
		t.Error("GetConfig should return a copy, not a pointer to the internal struct")
	}
}

func TestGetDefaultLibraryPath(t *testing.T) {
	path, err := GetDefaultLibraryPath()
	if err != nil {
		t.Fatalf("GetDefaultLibraryPath failed: %v", err)
	}
	if path == "" {
		t.Error("Expected a non-empty path")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("Expected absolute path, got %s", path)
	}
}
