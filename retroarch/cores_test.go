package retroarch

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-romm-sync/constants"
)

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

func TestGetCoresForExt(t *testing.T) {
	cores := GetCoresForExt(".sfc")
	if len(cores) == 0 {
		t.Errorf("Expected at least one core for .sfc, got none")
	}
	if cores[0] != "snes9x_libretro" {
		t.Errorf("Expected default core snes9x_libretro for .sfc, got %s", cores[0])
	}
	// Multiple cores should be offered
	if len(cores) < 2 {
		t.Errorf("Expected multiple cores for .sfc, got %d", len(cores))
	}
	cores = GetCoresForExt(".unknown")
	if len(cores) != 0 {
		t.Errorf("Expected empty slice for unknown ext, got %v", cores)
	}
}

func TestGetCoresForPlatform(t *testing.T) {
	cores := GetCoresForPlatform("gb")
	if len(cores) == 0 {
		t.Errorf("Expected at least one core for platform gb, got none")
	}
	if cores[0] != "gambatte_libretro" {
		t.Errorf("Expected default core architecture gambatte_libretro for platform gb, got %s", cores[0])
	}
	// Case-insensitivity check
	cores = GetCoresForPlatform("GB")
	if len(cores) == 0 || cores[0] != "gambatte_libretro" {
		t.Errorf("Expected GetCoresForPlatform to be case-insensitive, but it failed for GB")
	}
	cores = GetCoresForPlatform("unknown_platform")
	if len(cores) != 0 {
		t.Errorf("Expected empty slice for unknown platform, got %v", cores)
	}
}

func TestGetCoresFromZip(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "zip_test")
	defer os.RemoveAll(tempDir)
	zipPath := filepath.Join(tempDir, "test.zip")

	// Create a zip with recognizable extensions
	f, _ := os.Create(zipPath)
	zw := zip.NewWriter(f)
	_, _ = zw.Create("game.gb")
	_, _ = zw.Create("readme.txt")
	_, _ = zw.Create("sub/game.gba")
	zw.Close()
	f.Close()

	cores := GetCoresFromZip(zipPath)
	// .gb -> gambatte_libretro, mgba_libretro, sameboy_libretro
	// .gba -> mgba_libretro, vba_next_libretro
	// Union should contain them all, gambatte first since it's the first recognized
	if len(cores) == 0 {
		t.Fatal("Expected to find cores in zip")
	}

	foundGambatte := false
	foundMgba := false
	for _, c := range cores {
		if c == "gambatte_libretro" {
			foundGambatte = true
		}
		if c == "mgba_libretro" {
			foundMgba = true
		}
	}

	if !foundGambatte {
		t.Error("Expected gambatte_libretro to be found for .gb")
	}
	if !foundMgba {
		t.Error("Expected mgba_libretro to be found for .gba")
	}
}

func TestDownloadCore_NotFound(t *testing.T) {
	ui := &MockUI{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	oldURL := buildbotBaseURL
	buildbotBaseURL = server.URL
	defer func() { buildbotBaseURL = oldURL }()

	err := DownloadCore(ui, "missing_core.so", "/tmp", "amd64")
	if err == nil {
		t.Fatal("Expected error for 404 core")
	}
	if !strings.Contains(err.Error(), "status 404") {
		t.Errorf("Expected 404 error message, got: %v", err)
	}
}

func TestUpdateAllCores(t *testing.T) {
	ui := &MockUI{}

	// Create a mock server that returns a valid dummy zip file
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")

		// Create a small valid zip in memory
		buf := new(bytes.Buffer)
		zw := zip.NewWriter(buf)
		// We use a dummy name inside the zip for testing
		f, _ := zw.Create("dummy_core.info")
		f.Write([]byte("info data"))
		zw.Close()

		w.Write(buf.Bytes())
	}))
	defer server.Close()

	oldURL := buildbotBaseURL
	buildbotBaseURL = server.URL
	defer func() { buildbotBaseURL = oldURL }()

	tempDir, err := os.MkdirTemp("", "update_all_cores")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	exePath := filepath.Join(tempDir, "retroarch")
	os.WriteFile(exePath, []byte("fake"), 0o755)

	coresDir := filepath.Join(tempDir, "cores")
	os.MkdirAll(coresDir, 0o755)

	overrideCoresDir = coresDir
	defer func() { overrideCoresDir = "" }()

	// Create a couple of mock core files
	ext := getCoreExt()
	os.WriteFile(filepath.Join(coresDir, "core1"+ext), []byte("dummy data"), 0o644)
	os.WriteFile(filepath.Join(coresDir, "core2"+ext), []byte("dummy data"), 0o644)
	os.WriteFile(filepath.Join(coresDir, "notacore.txt"), []byte("dummy data"), 0o644)

	err = UpdateAllCores(ui, exePath)
	if err != nil {
		t.Fatalf("UpdateAllCores failed: %v", err)
	}

	// We expect EventPlayStatus to be emitted indicating success
	msgs := ui.GetEventMsgs(constants.EventPlayStatus)
	foundSuccess := false
	for _, msg := range msgs {
		if msg == "Finished updating 2 cores." {
			foundSuccess = true
			break
		}
	}
	if !foundSuccess {
		t.Errorf("Expected 'Finished updating 2 cores.', got messages: %v", msgs)
	}
}

func TestDSDefaultCore(t *testing.T) {
	cores := GetCoresForExt(".nds")
	if len(cores) < 2 || cores[0] != constants.CoreMelonDSDS || cores[1] != constants.CoreNooDS {
		t.Errorf("Expected default cores [%s, %s] for .nds, got %v", constants.CoreMelonDSDS, constants.CoreNooDS, cores)
	}

	cores = GetCoresForPlatform("nds")
	if len(cores) < 2 || cores[0] != constants.CoreMelonDSDS || cores[1] != constants.CoreNooDS {
		t.Errorf("Expected default cores [%s, %s] for platform nds, got %v", constants.CoreMelonDSDS, constants.CoreNooDS, cores)
	}
}
