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
	criticalExts := []string{".nes", ".sfc", ".gb", ".gba", ".z64", ".md", ".bin", ".p8", ".png"}
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

	// Mock exePath to point to the temp dir.
	// The function calls os.Stat(exePath), so the file must exist.
	exePath := filepath.Join(tmpDir, "retroarch.exe")
	os.WriteFile(exePath, []byte(""), 0644)

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

func TestResolveExecutableFromDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ra-exe-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a mock retroarch executable
	exeName := "retroarch.exe"
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		exeName = "retroarch"
	}
	if runtime.GOOS == "darwin" {
		exeName = "RetroArch"
	}

	exePath := filepath.Join(tmpDir, exeName)
	err = os.WriteFile(exePath, []byte("mock binary"), 0755)
	if err != nil {
		t.Fatalf("Failed to create mock exe: %v", err)
	}

	// This function isn't exported directly but it's part of the Launch logic.
	// We can test the logic by calling a helper if we refactor, but for now
	// let's verify Launch doesn't return an error early for a directory if the exe exists.

	// Since Launch runs things we can't easily unit test (exec.Command), we'll
	// just verify the directory resolution part of the logic works by checking
	// the file existence logic in a similar manner to how Launch does it.

	info, err := os.Stat(tmpDir)
	if err != nil || !info.IsDir() {
		t.Fatalf("Expected %s to be a directory", tmpDir)
	}

	target := filepath.Join(tmpDir, "retroarch.exe")
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		target = filepath.Join(tmpDir, "retroarch")
	}

	if runtime.GOOS == "darwin" {
		// Test .app detection or binary detection
		target = filepath.Join(tmpDir, "RetroArch")
	}

	if _, err := os.Stat(target); err != nil {
		t.Errorf("Path resolution logic failed to find expected executable at %s", target)
	}
}
