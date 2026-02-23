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
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to executable dir if home is not available
		exePath, err := os.Executable()
		if err != nil {
			exePath = "."
		}
		configPath := filepath.Join(filepath.Dir(exePath), "config.json")
		return &ConfigManager{
			ConfigPath: configPath,
			Config:     &types.AppConfig{},
		}
	}
	configPath := filepath.Join(home, ".go-romm-sync", "config", "config.json")

	return &ConfigManager{
		ConfigPath: configPath,
		Config:     &types.AppConfig{},
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
func (cm *ConfigManager) Save(newConfig *types.AppConfig) error {
	cm.Mu.Lock()
	defer cm.Mu.Unlock()

	*cm.Config = *newConfig

	// Ensure directory exists
	dir := filepath.Dir(cm.ConfigPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cm.Config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cm.ConfigPath, data, 0o644)
}

// GetDefaultLibraryPath returns the cross-platform default library path
func GetDefaultLibraryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, "Go-RomM-Sync", "Library"), nil
}

// createDefault generates a dummy config file if none exists
func (cm *ConfigManager) createDefault() error {
	defaultLibraryPath, _ := GetDefaultLibraryPath()
	if defaultLibraryPath == "" {
		defaultLibraryPath = filepath.Join(".", "Library")
	}

	defaultConfig := types.AppConfig{
		RommHost:            "",
		Username:            "",
		Password:            "",
		LibraryPath:         defaultLibraryPath,
		RetroArchPath:       "",
		RetroArchExecutable: "",
	}
	cm.Config = &defaultConfig

	fmt.Println("Config file not found. Creating default at:", cm.ConfigPath)

	// Create the directory if it doesn't exist
	dir := filepath.Dir(cm.ConfigPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cm.Config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cm.ConfigPath, data, 0o644)
}
