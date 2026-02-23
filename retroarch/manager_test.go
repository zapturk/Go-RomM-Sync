package retroarch

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestCoreMap(t *testing.T) {
	if CoreMap[".sfc"] != "snes9x_libretro" {
		t.Errorf("Expected snes9x_libretro for .sfc")
	}
	if CoreMap[".nes"] != "nestopia_libretro" {
		t.Errorf("Expected nestopia_libretro for .nes")
	}
}

func TestUnzipCore(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "unzip_test")
	defer os.RemoveAll(tempDir)

	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	f, _ := w.Create("test.txt")
	f.Write([]byte("hello"))
	w.Close()

	zipPath := filepath.Join(tempDir, "test.zip")
	os.WriteFile(zipPath, buf.Bytes(), 0o644)

	destDir := filepath.Join(tempDir, "dest")
	os.MkdirAll(destDir, 0o755)

	err := unzipCore(zipPath, destDir)
	if err != nil {
		t.Fatalf("unzipCore failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(destDir, "test.txt"))
	if err != nil {
		t.Fatalf("Failed to read unzipped file: %v", err)
	}
	if string(content) != "hello" {
		t.Errorf("Expected hello, got %s", string(content))
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

type MockUI struct{}

func (m *MockUI) LogInfof(format string, args ...interface{})      {}
func (m *MockUI) LogErrorf(format string, args ...interface{})     {}
func (m *MockUI) EventsEmit(eventName string, args ...interface{}) {}
func (m *MockUI) WindowHide()                                      {}
func (m *MockUI) WindowShow()                                      {}
func (m *MockUI) WindowUnminimise()                                {}

func TestLaunch_Errors(t *testing.T) {
	ui := &MockUI{}

	// Test missing exe
	err := Launch(ui, "/non/existent/retroarch", "rom.sfc", "", "")
	if err == nil {
		t.Error("Expected error for non-existent executable")
	}

	// Test missing core map
	tempDir, _ := os.MkdirTemp("", "launch_err")
	defer os.RemoveAll(tempDir)
	exePath := filepath.Join(tempDir, "retroarch")
	os.WriteFile(exePath, []byte("fake"), 0o755)

	err = Launch(ui, exePath, "rom.unknown", "", "")
	if err == nil {
		t.Error("Expected error for unknown extension")
	}
}

func TestLaunch_Zip(t *testing.T) {
	ui := &MockUI{}
	tempDir, _ := os.MkdirTemp("", "launch_zip")
	defer os.RemoveAll(tempDir)

	exePath := filepath.Join(tempDir, "retroarch")
	os.WriteFile(exePath, []byte("fake"), 0o755)

	// Create fake zip
	zipPath := filepath.Join(tempDir, "game.zip")
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	f, _ := w.Create("game.sfc")
	f.Write([]byte("rom data"))
	w.Close()
	os.WriteFile(zipPath, buf.Bytes(), 0o644)

	// Launch should return error because core won't be found (on non-supported systems for download or just missing)
	// But it should at least get past ZIP handling.
	// Actually, we want to test that it correctly identifies the core and formats the path.
	// Since Launch returns immediately after starting goroutine (if all pre-checks pass), we just check it doesn't return early error.

	err := Launch(ui, exePath, zipPath, "", "")
	// It might error because coresDir/cores/... missing, which is fine, we just want to see it gets there.
	if err != nil && !strings.Contains(err.Error(), "emulator core not found") {
		t.Errorf("Unexpected error during zip launch: %v", err)
	}
}

func TestLaunch_Pico8(t *testing.T) {
	ui := &MockUI{}
	tempDir, _ := os.MkdirTemp("", "launch_p8")
	defer os.RemoveAll(tempDir)

	exePath := filepath.Join(tempDir, "retroarch")
	os.WriteFile(exePath, []byte("fake"), 0o755)

	p8Path := filepath.Join(tempDir, "game.png")
	os.WriteFile(p8Path, []byte("png data"), 0o644)

	err := Launch(ui, exePath, p8Path, "", "")
	if err != nil && !strings.Contains(err.Error(), "emulator core not found") {
		t.Errorf("Unexpected error during pico8 launch: %v", err)
	}
}

func TestDownloadCore(t *testing.T) {
	ui := &MockUI{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock a ZIP response
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)
		f, _ := zw.Create("test_core.so")
		f.Write([]byte("core data"))
		zw.Close()
		w.Write(buf.Bytes())
	}))
	defer server.Close()

	// Since DownloadCore constructs its own URL, we can't easily point it to the test server
	// without changing the code. However, we can test unzipCore directly (already done)
	// and we can test DownloadCore's error handling for unsupported OS/Arch.

	err := DownloadCore(ui, "core.so", "/tmp", "invalid-arch")
	if err == nil {
		t.Error("Expected error for invalid arch")
	}
}

func TestLaunch_ExeDir(t *testing.T) {
	ui := &MockUI{}
	tempDir, _ := os.MkdirTemp("", "launch_exe_dir")
	defer os.RemoveAll(tempDir)

	// Create fake retroarch inside directory
	var exeName string
	if runtime.GOOS == "windows" {
		exeName = "retroarch.exe"
	} else {
		exeName = "retroarch"
	}
	exePath := filepath.Join(tempDir, exeName)
	os.WriteFile(exePath, []byte("fake"), 0o755)

	err := Launch(ui, tempDir, "rom.sfc", "", "")
	if err != nil && !strings.Contains(err.Error(), "emulator core not found") {
		t.Errorf("Unexpected error during exe dir launch: %v", err)
	}
}

func TestLaunch_AppBundle(t *testing.T) {
	if runtime.GOOS != constants.OSDarwin {
		t.Skip("Skipping macOS specific test")
	}
	ui := &MockUI{}
	tempDir, _ := os.MkdirTemp("", "launch_app_bundle")
	defer os.RemoveAll(tempDir)

	appPath := filepath.Join(tempDir, "RetroArch.app")
	os.MkdirAll(appPath, 0o755)

	err := Launch(ui, appPath, "rom.sfc", "", "")
	// Should at least pass the directory check and fail on core/binary lookup
	if err != nil && strings.Contains(err.Error(), "retroarch executable not found in directory") {
		t.Errorf("Failed to resolve .app bundle: %v", err)
	}
}
