package configsrv

import (
	"errors"
	"go-romm-sync/types"
	"testing"
)

// MockConfigManager implements ConfigManager interface
type MockConfigManager struct {
	Config      types.AppConfig
	SaveCalled  bool
	SaveError   error
	GetConfigFn func() types.AppConfig
}

func (m *MockConfigManager) ConfigGetConfig() types.AppConfig {
	if m.GetConfigFn != nil {
		return m.GetConfigFn()
	}
	return m.Config
}

func (m *MockConfigManager) ConfigSave(cfg types.AppConfig) error {
	m.SaveCalled = true
	m.Config = cfg
	return m.SaveError
}

// MockUIProvider implements UIProvider interface
type MockUIProvider struct {
	SelectedFile string
	SelectedDir  string
	Error        error
}

func (m *MockUIProvider) OpenFileDialog(title string, filters []string) (string, error) {
	return m.SelectedFile, m.Error
}

func (m *MockUIProvider) OpenDirectoryDialog(title string) (string, error) {
	return m.SelectedDir, m.Error
}

func TestNew(t *testing.T) {
	cm := &MockConfigManager{}
	ui := &MockUIProvider{}
	s := New(cm, ui)

	if s.cm != cm {
		t.Errorf("Expected cm to be set")
	}
	if s.ui != ui {
		t.Errorf("Expected ui to be set")
	}
}

func TestGetConfig(t *testing.T) {
	expected := types.AppConfig{RommHost: "http://localhost:8080"}
	cm := &MockConfigManager{Config: expected}
	s := New(cm, nil)

	actual := s.GetConfig()
	if actual.RommHost != expected.RommHost {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func TestSaveConfig(t *testing.T) {
	cm := &MockConfigManager{
		Config: types.AppConfig{RommHost: "old-host"},
	}
	s := New(cm, nil)

	newCfg := types.AppConfig{
		RommHost: "new-host",
		Username: "new-user",
	}

	msg, hostChanged := s.SaveConfig(newCfg)

	if !hostChanged {
		t.Errorf("Expected hostChanged to be true")
	}
	if msg != "Configuration saved successfully!" {
		t.Errorf("Unexpected message: %s", msg)
	}
	if cm.Config.RommHost != "new-host" {
		t.Errorf("Expected RommHost to be updated")
	}
	if cm.Config.Username != "new-user" {
		t.Errorf("Expected Username to be updated")
	}
}

func TestSaveConfig_Error(t *testing.T) {
	cm := &MockConfigManager{
		SaveError: errors.New("save failed"),
	}
	s := New(cm, nil)

	msg, hostChanged := s.SaveConfig(types.AppConfig{})

	if hostChanged {
		t.Errorf("Expected hostChanged to be false")
	}
	if msg != "Error saving config: save failed" {
		t.Errorf("Unexpected error message: %s", msg)
	}
}

func TestSelectRetroArchExecutable(t *testing.T) {
	cm := &MockConfigManager{}
	ui := &MockUIProvider{SelectedFile: "/path/to/retroarch"}
	s := New(cm, ui)

	selected, err := s.SelectRetroArchExecutable()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if selected != "/path/to/retroarch" {
		t.Errorf("Expected %s, got %s", "/path/to/retroarch", selected)
	}
	if cm.Config.RetroArchPath != "/path/to/retroarch" {
		t.Errorf("Expected config to be updated")
	}
}

func TestSelectLibraryPath(t *testing.T) {
	cm := &MockConfigManager{}
	ui := &MockUIProvider{SelectedDir: "/path/to/library"}
	s := New(cm, ui)

	selected, err := s.SelectLibraryPath()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if selected != "/path/to/library" {
		t.Errorf("Expected %s, got %s", "/path/to/library", selected)
	}
	if cm.Config.LibraryPath != "/path/to/library" {
		t.Errorf("Expected config to be updated")
	}
}
