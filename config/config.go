package config

import (
	"encoding/json"
	"fmt"
	"go-romm-sync/types"
	"os"
	"path/filepath"
	"sync"
)

// ConfigManager handles loading/saving
type ConfigManager struct {
	Config     *types.AppConfig
	ConfigPath string
	Mu         sync.RWMutex // Thread-safety for UI reads/writes
}

// NewConfigManager initializes the manager and determines the file path
func NewConfigManager() *ConfigManager {
	// Strategy: Look for config.json next to the executable
	exePath, err := os.Executable()
	if err != nil {
		exePath = "."
	}
	configPath := filepath.Join(filepath.Dir(exePath), "config.json")

	return &ConfigManager{
		ConfigPath: configPath,
		Config:     &types.AppConfig{}, // Empty default
	}
}

// Load reads the config from disk
func (cm *ConfigManager) Load() error {
	cm.Mu.Lock()
	defer cm.Mu.Unlock()

	// 1. Check if file exists
	if _, err := os.Stat(cm.ConfigPath); os.IsNotExist(err) {
		return cm.createDefault()
	}

	// 2. Read bytes
	data, err := os.ReadFile(cm.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// 3. Unmarshal
	if err := json.Unmarshal(data, cm.Config); err != nil {
		return fmt.Errorf("failed to parse config json: %w", err)
	}

	return nil
}

// GetConfig returns a copy of the current config (Thread-Safe)
func (cm *ConfigManager) GetConfig() types.AppConfig {
	cm.Mu.RLock()
	defer cm.Mu.RUnlock()
	return *cm.Config
}

// Save writes the current config to disk
func (cm *ConfigManager) Save(newConfig types.AppConfig) error {
	cm.Mu.Lock()
	defer cm.Mu.Unlock()

	cm.Config = &newConfig

	data, err := json.MarshalIndent(cm.Config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cm.ConfigPath, data, 0644)
}

// createDefault generates a dummy config file if none exists
func (cm *ConfigManager) createDefault() error {
	defaultConfig := types.AppConfig{
		RommHost:            "",
		Username:            "",
		Password:            "",
		LibraryPath:         "",
		RetroArchPath:       "",
		RetroArchExecutable: "",
	}
	cm.Config = &defaultConfig

	fmt.Println("Config file not found. Creating default at:", cm.ConfigPath)

	// Create the directory if it doesn't exist
	dir := filepath.Dir(cm.ConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cm.Config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cm.ConfigPath, data, 0644)
}
