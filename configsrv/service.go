package configsrv

import (
	"fmt"
	"go-romm-sync/types"
	"path/filepath"
	"runtime"
	"strings"
)

// ConfigManager defines the interface for managing the app configuration.
type ConfigManager interface {
	ConfigGetConfig() types.AppConfig
	ConfigSave(cfg types.AppConfig) error
}

// UIProvider defines the UI interactions needed for configuration.
type UIProvider interface {
	OpenFileDialog(title string, filters []string) (string, error)
	OpenDirectoryDialog(title string) (string, error)
}

// Service handles configuration-related logic.
type Service struct {
	cm ConfigManager
	ui UIProvider
}

// New creates a new Config service.
func New(cm ConfigManager, ui UIProvider) *Service {
	return &Service{
		cm: cm,
		ui: ui,
	}
}

// GetConfig returns the current configuration.
func (s *Service) GetConfig() types.AppConfig {
	return s.cm.ConfigGetConfig()
}

// SaveConfig merges and saves the configuration.
func (s *Service) SaveConfig(cfg types.AppConfig) (string, bool) {
	current := s.cm.ConfigGetConfig()
	oldHost := current.RommHost

	updateIfNotEmpty(&current.RommHost, cfg.RommHost)
	updateIfNotEmpty(&current.Username, cfg.Username)
	updateIfNotEmpty(&current.Password, cfg.Password)
	updateIfNotEmpty(&current.LibraryPath, cfg.LibraryPath)

	// Robust RetroArch path handling: Derive directory from executable if provided
	if cfg.RetroArchExecutable != "" {
		current.RetroArchExecutable = cfg.RetroArchExecutable
		current.RetroArchPath = filepath.Dir(cfg.RetroArchExecutable)
	} else if cfg.RetroArchPath != "" {
		// If only path is provided, check if it's an executable
		ext := filepath.Ext(cfg.RetroArchPath)
		if ext == ".exe" || ext == ".app" || strings.HasSuffix(cfg.RetroArchPath, "retroarch") {
			current.RetroArchExecutable = cfg.RetroArchPath
			current.RetroArchPath = filepath.Dir(cfg.RetroArchPath)
		} else {
			current.RetroArchPath = cfg.RetroArchPath
		}
	}

	updateIfNotEmpty(&current.CheevosUsername, cfg.CheevosUsername)
	updateIfNotEmpty(&current.CheevosPassword, cfg.CheevosPassword)

	if err := s.cm.ConfigSave(current); err != nil {
		return fmt.Sprintf("Error saving config: %s", err.Error()), false
	}

	hostChanged := current.RommHost != oldHost
	return "Configuration saved successfully!", hostChanged
}

func updateIfNotEmpty(target *string, value string) {
	if value != "" {
		*target = value
	}
}

// SelectRetroArchExecutable handles the selection of the RetroArch binary.
func (s *Service) SelectRetroArchExecutable() (string, error) {
	filters := []string{"*.*"}
	if runtime.GOOS != "darwin" {
		filters = append(filters, "*.exe;*.app;retroarch")
	}

	selectedFile, err := s.ui.OpenFileDialog("Select RetroArch Executable", filters)
	if err != nil {
		return "", err
	}

	if selectedFile != "" {
		cfg := s.cm.ConfigGetConfig()
		cfg.RetroArchExecutable = selectedFile
		cfg.RetroArchPath = filepath.Dir(selectedFile)
		if err = s.cm.ConfigSave(cfg); err != nil {
			return "", fmt.Errorf("failed to save config: %w", err)
		}
	}

	return selectedFile, nil
}

// SelectLibraryPath handles the selection of the library directory.
func (s *Service) SelectLibraryPath() (string, error) {
	selectedDir, err := s.ui.OpenDirectoryDialog("Select ROM Library Directory")
	if err != nil {
		return "", err
	}

	if selectedDir != "" {
		cfg := s.cm.ConfigGetConfig()
		cfg.LibraryPath = selectedDir
		if err = s.cm.ConfigSave(cfg); err != nil {
			return "", fmt.Errorf("failed to save config: %w", err)
		}
	}

	return selectedDir, nil
}
