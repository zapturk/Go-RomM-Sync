package firmware

import (
	"archive/zip"
	"bytes"
	"go-romm-sync/config"
	"go-romm-sync/rommsrv"
	"go-romm-sync/types"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

type mockRommConfig struct{}

func (m mockRommConfig) GetRomMHost() string      { return "http://localhost" }
func (m mockRommConfig) GetUsername() string      { return "user" }
func (m mockRommConfig) GetPassword() string      { return "pass" }
func (m mockRommConfig) GetClientToken() string   { return "token" }

type mockTransport struct {
	roundTrip func(*http.Request) (*http.Response, error)
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.roundTrip(req)
}

type MockUIProvider struct{}

func (m *MockUIProvider) LogInfof(format string, args ...interface{})          {}
func (m *MockUIProvider) LogErrorf(format string, args ...interface{})         {}
func (m *MockUIProvider) EventsEmit(eventName string, args ...interface{})     {}

func TestDownloadFirmware(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "firmware_test")
	defer os.RemoveAll(tempDir)

	cm := config.NewConfigManager()
	cm.ConfigPath = filepath.Join(tempDir, "config.json")
	cm.Config = &types.AppConfig{LibraryPath: tempDir}

	rommSrv := rommsrv.New(mockRommConfig{})
	rommSrv.GetClient().FileClient.Transport = &mockTransport{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("dummy firmware"))),
			}, nil
		},
	}

	fw := &types.Firmware{
		ID:       1,
		FileName: "scd_v2.21.bin",
		MD5Hash:  "baca1df271d7c11fe50087c0358f4eb5",
	}

	ui := &MockUIProvider{}
	s := New(cm, rommSrv, ui)

	err := s.DownloadFirmware("segacd", fw)
	if err != nil {
		t.Fatalf("DownloadFirmware failed: %v", err)
	}

	// Should be renamed to bios_CD_U.bin based on MD5
	destPath := filepath.Join(tempDir, "bios", "bios_CD_U.bin")
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("Expected BIOS file to be renamed to bios_CD_U.bin, but not found at %s", destPath)
	}
}

func TestDownloadFirmware_Compressed(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "firmware_test_zip")
	defer os.RemoveAll(tempDir)

	cm := config.NewConfigManager()
	cm.ConfigPath = filepath.Join(tempDir, "config.json")
	cm.Config = &types.AppConfig{LibraryPath: tempDir}

	// Create a dummy ZIP file
	zipBuffer := new(bytes.Buffer)
	zw := zip.NewWriter(zipBuffer)

	f, _ := zw.Create("internal_bios.bin")
	f.Write([]byte("some bios content"))
	zw.Close()

	rommSrv := rommsrv.New(mockRommConfig{})
	rommSrv.GetClient().FileClient.Transport = &mockTransport{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(zipBuffer.Bytes())),
			}, nil
		},
	}

	fw := &types.Firmware{
		ID:       1,
		FileName: "bios_package.zip",
	}

	ui := &MockUIProvider{}
	s := New(cm, rommSrv, ui)

	err := s.DownloadFirmware("nds", fw)
	if err != nil {
		t.Fatalf("DownloadFirmware failed: %v", err)
	}

	// Should be extracted to bios/internal_bios.bin
	destPath := filepath.Join(tempDir, "bios", "internal_bios.bin")
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("Expected extracted BIOS file to be found at %s", destPath)
	}
}

func TestCleanupFirmware(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "firmware_test_cleanup")
	defer os.RemoveAll(tempDir)

	cm := config.NewConfigManager()
	cm.ConfigPath = filepath.Join(tempDir, "config.json")
	cm.Config = &types.AppConfig{LibraryPath: tempDir}

	s := New(cm, nil, &MockUIProvider{})

	biosDir := filepath.Join(tempDir, "bios")
	os.MkdirAll(biosDir, 0o755)

	// Create a dummy BIOS file for Sega CD
	biosPath := filepath.Join(biosDir, "bios_CD_U.bin")
	os.WriteFile(biosPath, []byte("fake bios"), 0o644)

	// Create a dummy BIOS file for PS1 (should stay)
	ps1Path := filepath.Join(biosDir, "scph5501.bin")
	os.WriteFile(ps1Path, []byte("fake ps1 bios"), 0o644)

	// Cleanup for Sega CD
	err := s.CleanupFirmware("segacd")
	if err != nil {
		t.Fatalf("CleanupFirmware failed: %v", err)
	}

	// Sega CD BIOS should be gone
	if _, err := os.Stat(biosPath); !os.IsNotExist(err) {
		t.Errorf("Expected Sega CD BIOS to be deleted")
	}

	// PS1 BIOS should still be there
	if _, err := os.Stat(ps1Path); err != nil {
		t.Errorf("Expected PS1 BIOS to still exist")
	}
}

func TestIsFirmwareDownloaded(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "firmware_test_is_downloaded")
	defer os.RemoveAll(tempDir)

	cm := config.NewConfigManager()
	cm.ConfigPath = filepath.Join(tempDir, "config.json")
	cm.Config = &types.AppConfig{LibraryPath: tempDir}

	s := New(cm, nil, &MockUIProvider{})

	biosDir := filepath.Join(tempDir, "bios")
	os.MkdirAll(biosDir, 0o755)

	fw := &types.Firmware{
		FileName: "scd_v2.21.bin",
		MD5Hash:  "baca1df271d7c11fe50087c0358f4eb5",
	}

	// 1. Not downloaded
	if s.IsFirmwareDownloaded("segacd", fw) {
		t.Errorf("Expected false for missing firmware")
	}

	// 2. Downloaded (canonical name)
	os.WriteFile(filepath.Join(biosDir, "bios_CD_U.bin"), []byte("data"), 0o644)
	if !s.IsFirmwareDownloaded("segacd", fw) {
		t.Errorf("Expected true for canonical firmware file")
	}
}
