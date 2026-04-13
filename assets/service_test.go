package assets

import (
	"fmt"
	"go-romm-sync/constants"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type MockConfigProvider struct {
	Host string
}

func (m *MockConfigProvider) GetRomMHost() string {
	return m.Host
}

type MockRomMClient struct {
	Host string
}

func (m *MockRomMClient) DownloadCover(url string) ([]byte, error) {
	if strings.Contains(url, "error") {
		return nil, fmt.Errorf("simulated download error")
	}
	if strings.HasSuffix(url, "snes.svg") {
		return []byte("<svg></svg>"), nil
	}
	if strings.HasSuffix(url, "snes.png") {
		return []byte("png data"), nil
	}
	if strings.HasSuffix(url, "snes_alt.svg") {
		return []byte("<svg></svg>"), nil
	}
	if strings.Contains(url, "cover.jpg") {
		return []byte("downloaded image data"), nil
	}
	if strings.HasSuffix(url, "image.png") {
		return []byte("png data"), nil
	}
	return nil, fmt.Errorf("not found")
}

type MockUIProvider struct{}

func (m *MockUIProvider) LogErrorf(format string, args ...interface{}) {}

func TestGetMimeType(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{".svg", "image/svg+xml"},
		{".ico", "image/x-icon"},
		{".png", "image/png"},
		{".jpg", "image/jpeg"},
		{".jpeg", "image/jpeg"},
		{".exe", "application/octet-stream"},
	}

	for _, tt := range tests {
		actual := getMimeType(tt.ext)
		if actual != tt.expected {
			t.Errorf("getMimeType(%s) = %s, expected %s", tt.ext, actual, tt.expected)
		}
	}
}

func TestGetCover_Cached(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home dir: %v", err)
	}
	cacheDir := filepath.Join(homeDir, constants.AppDir, constants.CacheDir, constants.CoversDir)
	os.MkdirAll(cacheDir, 0o755)

	romID := uint(9999)
	cachePath := filepath.Join(cacheDir, "9999.jpg")
	os.WriteFile(cachePath, []byte("dummy image data"), 0o644)
	defer os.Remove(cachePath)

	s := &Service{} // client not needed for cached path
	data, err := s.GetCover(romID, "http://example.com/cover.jpg")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !strings.HasPrefix(data, "data:image/jpeg;base64,") {
		t.Errorf("Expected JPEG data URI, got %s", data)
	}
}

func TestGetCover_Download(t *testing.T) {
	cfg := &MockConfigProvider{Host: "http://localhost"}
	client := &MockRomMClient{Host: "http://localhost"}
	ui := &MockUIProvider{}
	s := New(cfg, client, ui)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home dir: %v", err)
	}
	cachePath := filepath.Join(homeDir, constants.AppDir, constants.CacheDir, constants.CoversDir, "1234.jpg")
	os.Remove(cachePath)
	defer os.Remove(cachePath)

	data, err := s.GetCover(1234, "/some/cover.jpg")
	if err != nil {
		t.Fatalf("GetCover failed: %v", err)
	}
	if !strings.HasPrefix(data, "data:image/jpeg;base64,") {
		t.Errorf("Expected JPEG data URI, got %s", data)
	}
}

func TestGetCover_PNG(t *testing.T) {
	cfg := &MockConfigProvider{Host: "http://localhost"}
	client := &MockRomMClient{Host: "http://localhost"}
	ui := &MockUIProvider{}
	s := New(cfg, client, ui)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home dir: %v", err)
	}
	cachePath := filepath.Join(homeDir, constants.AppDir, constants.CacheDir, constants.CoversDir, "5678.png")
	os.Remove(cachePath)
	defer os.Remove(cachePath)

	data, err := s.GetCover(5678, "http://localhost/image.png")
	if err != nil {
		t.Fatalf("GetCover failed: %v", err)
	}
	if !strings.HasPrefix(data, "data:image/png;base64,") {
		t.Errorf("Expected PNG data URI, got %s", data)
	}
}

func TestGetPlatformCover_Download(t *testing.T) {
	cfg := &MockConfigProvider{Host: "http://localhost"}
	client := &MockRomMClient{Host: "http://localhost"}
	ui := &MockUIProvider{}
	s := New(cfg, client, ui)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home dir: %v", err)
	}
	cacheDir := filepath.Join(homeDir, constants.AppDir, constants.CacheDir, constants.PlatformsDir)
	for _, ext := range []string{".svg", ".ico", ".png", ".jpg"} {
		os.Remove(filepath.Join(cacheDir, "1"+ext))
	}
	defer func() {
		for _, ext := range []string{".svg", ".ico", ".png", ".jpg"} {
			os.Remove(filepath.Join(cacheDir, "1"+ext))
		}
	}()

	data, err := s.GetPlatformCover(1, "snes")
	if err != nil {
		t.Fatalf("GetPlatformCover failed: %v", err)
	}
	if !strings.HasPrefix(data, "data:image/svg+xml;base64,") {
		t.Errorf("Expected SVG data URI, got %s", data)
	}
}

func TestToDataURI(t *testing.T) {
	data := []byte("hello")
	expectedMime := "image/png"
	// base64.StdEncoding.EncodeToString([]byte("hello")) is "aGVsbG8="
	expectedPrefix := "data:" + expectedMime + ";base64,"

	actual := toDataURI(data, ".png")
	if !strings.HasPrefix(actual, expectedPrefix) {
		t.Errorf("toDataURI failed, expected prefix %s, got %s", expectedPrefix, actual)
	}
	if !strings.Contains(actual, "aGVsbG8=") {
		t.Errorf("toDataURI failed, expected encoded data aGVsbG8=, got %s", actual)
	}
}

func TestTryGetPlatformCoverFromCache(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home dir: %v", err)
	}
	cacheDir := filepath.Join(homeDir, constants.AppDir, constants.CacheDir, "test_platforms")
	os.MkdirAll(cacheDir, 0o755)
	defer os.RemoveAll(cacheDir)

	platformID := uint(777)
	exts := []string{".png", ".svg"}

	// 1. Test miss
	s := &Service{}
	data, ext := s.tryGetPlatformCoverFromCache(cacheDir, platformID, exts)
	if data != nil || ext != "" {
		t.Errorf("Expected nil data and empty ext for cache miss, got %v and %s", data, ext)
	}

	// 2. Test hit (png)
	pngPath := filepath.Join(cacheDir, "777.png")
	os.WriteFile(pngPath, []byte("png-data"), 0o644)
	data, ext = s.tryGetPlatformCoverFromCache(cacheDir, platformID, exts)
	if string(data) != "png-data" || ext != ".png" {
		t.Errorf("Expected png-data and .png, got %s and %s", string(data), ext)
	}

	// 3. Test priority (svg before png if listed first)
	svgPath := filepath.Join(cacheDir, "777.svg")
	os.WriteFile(svgPath, []byte("svg-data"), 0o644)
	data, ext = s.tryGetPlatformCoverFromCache(cacheDir, platformID, []string{".svg", ".png"})
	if string(data) != "svg-data" || ext != ".svg" {
		t.Errorf("Expected svg-data and .svg, got %s and %s", string(data), ext)
	}
}

func TestTryDownloadPlatformCover(t *testing.T) {
	cfg := &MockConfigProvider{Host: "http://localhost"}
	client := &MockRomMClient{Host: "http://localhost"}
	ui := &MockUIProvider{}
	s := New(cfg, client, ui)
	exts := []string{".png", ".svg"}

	// 1. Primary slug hit
	data, ext := s.tryDownloadPlatformCover("snes", exts)
	if string(data) != "png data" || ext != ".png" {
		t.Errorf("Expected png data and .png, got '%s' and '%s'", string(data), ext)
	}

	// 2. Alt slug hit
	data, ext = s.tryDownloadPlatformCover("snes-alt", exts)
	if string(data) != "<svg></svg>" || ext != ".svg" {
		t.Errorf("Expected <svg></svg> and .svg, got '%s' and '%s'", string(data), ext)
	}

	// 3. Miss
	data, ext = s.tryDownloadPlatformCover("error", exts)
	if data != nil || ext != "" {
		t.Errorf("Expected nil data and empty ext for download miss, got %v and %s", data, ext)
	}
}
