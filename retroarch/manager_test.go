package retroarch

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGetCoreExt(t *testing.T) {
	ext := getCoreExt()
	switch runtime.GOOS {
	case "windows":
		if ext != ".dll" {
			t.Errorf("Expected .dll for windows, got %s", ext)
		}
	case "darwin":
		if ext != ".dylib" {
			t.Errorf("Expected .dylib for darwin, got %s", ext)
		}
	case "linux":
		if ext != ".so" {
			t.Errorf("Expected .so for linux, got %s", ext)
		}
	}
}

func TestCoreMapCoverage(t *testing.T) {
	// Verify some critical mappings exist
	criticalExts := []string{".nes", ".sfc", ".gb", ".gba", ".z64", ".md", ".bin"}
	for _, ext := range criticalExts {
		if _, ok := CoreMap[ext]; !ok {
			t.Errorf("Missing core mapping for critical extension: %s", ext)
		}
	}
}

func TestCoreMapUniqueness(t *testing.T) {
	// This is more of a sanity check that we don't have empty strings as cores
	for ext, core := range CoreMap {
		if core == "" {
			t.Errorf("Extension %s mapped to empty core name", ext)
		}
	}
}
func TestClearCheevosToken(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ra-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "retroarch.cfg")
	initialContent := `
savefile_directory = "C:\Saves"
cheevos_enable = "true"
cheevos_token = "some-long-session-token-12345"
cheevos_token_extra = "should stay"
cheevos_username = "testuser"
`
	err = os.WriteFile(configPath, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write mock config: %v", err)
	}

	// Mock exePath to point to the temp dir
	exePath := filepath.Join(tmpDir, "retroarch.exe")

	err = ClearCheevosToken(exePath)
	if err != nil {
		t.Fatalf("ClearCheevosToken returned error: %v", err)
	}

	// Verify content
	updatedContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read updated config: %v", err)
	}

	sContent := string(updatedContent)
	if !strings.Contains(sContent, `cheevos_token = ""`) {
		t.Errorf("Expected cheevos_token to be cleared, but got:\n%s", sContent)
	}
	if strings.Contains(sContent, "some-long-session-token-12345") {
		t.Errorf("Old token still present in config:\n%s", sContent)
	}
	if !strings.Contains(sContent, `cheevos_token_extra = "should stay"`) {
		t.Errorf("Other settings were incorrectly modified:\n%s", sContent)
	}
}
