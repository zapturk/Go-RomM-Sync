package retroarch

import (
	"runtime"
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
