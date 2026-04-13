package retroarch

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"go-romm-sync/constants"
)

func TestGetCoreExt(t *testing.T) {
	ext := getCoreExt()
	switch runtime.GOOS {
	case constants.OSWindows:
		if ext != ".dll" {
			t.Errorf("Expected .dll, got %s", ext)
		}
	case constants.OSDarwin:
		if ext != ".dylib" {
			t.Errorf("Expected .dylib, got %s", ext)
		}
	default:
		if ext != ".so" {
			t.Errorf("Expected .so, got %s", ext)
		}
	}
}

func TestGetCoresDir(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home dir: %v", err)
	}

	switch runtime.GOOS {
	case constants.OSDarwin:
		expected := filepath.Join(homeDir, "Library", "Application Support", "RetroArch", "cores")
		if dir := getCoresDir("/Applications/RetroArch.app/Contents/MacOS"); dir != expected {
			t.Errorf("Expected macOS core dir %s, got %s", expected, dir)
		}
	case constants.OSLinux:
		// Test Native
		expectedNative := filepath.Join(homeDir, ".config", "retroarch", "cores")
		if dir := getCoresDir("/usr/bin"); dir != expectedNative {
			t.Errorf("Expected Linux native core dir %s, got %s", expectedNative, dir)
		}

		// Test Snap
		expectedSnap := filepath.Join(homeDir, "snap", "retroarch", "current", ".config", "retroarch", "cores")
		if dir := getCoresDir("/snap/retroarch/current/usr/bin"); dir != expectedSnap {
			t.Errorf("Expected Snap core dir %s, got %s", expectedSnap, dir)
		}

		// Test Flatpak
		expectedFlatpak := filepath.Join(homeDir, ".var", "app", "org.libretro.RetroArch", "config", "retroarch", "cores")
		if dir := getCoresDir("/var/lib/flatpak/app/org.libretro.RetroArch/current/active/files/bin"); dir != expectedFlatpak {
			t.Errorf("Expected Flatpak core dir %s, got %s", expectedFlatpak, dir)
		}
	case constants.OSWindows:
		expectedWin := filepath.Join("C:", string(filepath.Separator), "RetroArch", "cores")
		if dir := getCoresDir("C:\\RetroArch"); dir != expectedWin {
			t.Errorf("Expected Windows core dir %s, got %s", expectedWin, dir)
		}
	}
}

func TestClearCheevosToken(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "cheevos_test")
	defer os.RemoveAll(tempDir)

	cfgPath := filepath.Join(tempDir, "retroarch.cfg")
	content := "cheevos_enable = \"true\"\ncheevos_token = \"abcdef123456\"\nother_setting = \"val\"\n"
	os.WriteFile(cfgPath, []byte(content), 0o644)

	err := ClearCheevosToken(tempDir)
	if err != nil {
		t.Fatalf("ClearCheevosToken failed: %v", err)
	}

	newContent, _ := os.ReadFile(cfgPath)
	if !bytes.Contains(newContent, []byte(`cheevos_token = ""`)) {
		t.Errorf("Expected cheevos_token to be cleared, got:\n%s", string(newContent))
	}
}
