package rommsrv

import (
	"go-romm-sync/constants"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MockConfigProvider implements ConfigProvider
type MockConfigProvider struct {
	Host string
}

func (m *MockConfigProvider) GetRomMHost() string {
	return m.Host
}

func (m *MockConfigProvider) GetUsername() string {
	return "user"
}

func (m *MockConfigProvider) GetPassword() string {
	return "pass"
}

func TestNew(t *testing.T) {
	cfg := &MockConfigProvider{Host: "http://localhost"}
	s := New(cfg)
	if s.client == nil {
		t.Errorf("Client not initialized")
	}
	if s.client.BaseURL != "http://localhost" {
		t.Errorf("Expected base URL http://localhost, got %s", s.client.BaseURL)
	}
}

func TestLogin_MissingConfig(t *testing.T) {
	cfg := &MockConfigProvider{Host: ""}
	s := New(cfg)
	_, err := s.Login()
	if err == nil {
		t.Errorf("Expected error for missing host")
	}
}

func TestLogin_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token": "valid-token"}`))
	}))
	defer server.Close()

	cfg := &MockConfigProvider{Host: server.URL}
	s := New(cfg)
	token, err := s.Login()
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if token != "valid-token" {
		t.Errorf("Expected valid-token, got %s", token)
	}
}

func TestGetLibrary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id": 1, "name": "Game 1"}]`))
	}))
	defer server.Close()

	cfg := &MockConfigProvider{Host: server.URL}
	s := New(cfg)
	s.client.Token = "test-token"

	games, _, err := s.GetLibrary(25, 0, 1)
	if err != nil {
		t.Fatalf("GetLibrary failed: %v", err)
	}
	if len(games) != 1 {
		t.Errorf("Expected 1 game, got %d", len(games))
	}
}

func TestGetPlatforms(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id": 1, "name": "Platform 1"}]`))
	}))
	defer server.Close()

	cfg := &MockConfigProvider{Host: server.URL}
	s := New(cfg)
	s.client.Token = "test-token"

	platforms, err := s.GetPlatforms(25, 0)
	if err != nil {
		t.Fatalf("GetPlatforms failed: %v", err)
	}
	if len(platforms) != 1 {
		t.Errorf("Expected 1 platform, got %d", len(platforms))
	}
}

func TestGetRom(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id": 1, "name": "Game 1"}`))
	}))
	defer server.Close()

	cfg := &MockConfigProvider{Host: server.URL}
	s := New(cfg)
	s.client.Token = "test-token"

	game, err := s.GetRom(1)
	if err != nil {
		t.Fatalf("GetRom failed: %v", err)
	}
	if game.ID != 1 {
		t.Errorf("Expected ID 1, got %d", game.ID)
	}
}

func TestGetServerSavesStates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "saves") {
			w.Write([]byte(`[{"id": 1, "filename": "save.srm"}]`))
		} else {
			w.Write([]byte(`[{"id": 1, "filename": "state.st0"}]`))
		}
	}))
	defer server.Close()

	cfg := &MockConfigProvider{Host: server.URL}
	s := New(cfg)
	s.client.Token = "test-token"

	saves, err := s.GetServerSaves(1)
	if err != nil {
		t.Fatalf("GetServerSaves failed: %v", err)
	}
	if len(saves) != 1 {
		t.Errorf("Expected 1 save, got %d", len(saves))
	}

	states, err := s.GetServerStates(1)
	if err != nil {
		t.Fatalf("GetServerStates failed: %v", err)
	}
	if len(states) != 1 {
		t.Errorf("Expected 1 state, got %d", len(states))
	}
}

func TestGetClient(t *testing.T) {
	cfg := &MockConfigProvider{Host: "http://localhost"}
	s := New(cfg)
	if s.GetClient() != s.client {
		t.Errorf("GetClient did not return the correct client")
	}
}

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
	homeDir, _ := os.UserHomeDir()
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write([]byte("downloaded image data"))
	}))
	defer server.Close()

	cfg := &MockConfigProvider{Host: server.URL}
	s := New(cfg)
	s.client.Token = "test-token" // Ensure cache is clean for this ID

	homeDir, _ := os.UserHomeDir()
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("png data"))
	}))
	defer server.Close()

	cfg := &MockConfigProvider{Host: server.URL}
	s := New(cfg)
	s.client.Token = "test-token"

	homeDir, _ := os.UserHomeDir()
	cachePath := filepath.Join(homeDir, ".go-romm-sync", "cache", "covers", "5678.png")
	os.Remove(cachePath)
	defer os.Remove(cachePath)

	data, err := s.GetCover(5678, server.URL+"/image.png")
	if err != nil {
		t.Fatalf("GetCover failed: %v", err)
	}
	if !strings.HasPrefix(data, "data:image/png;base64,") {
		t.Errorf("Expected PNG data URI, got %s", data)
	}
}

func TestGetPlatformCover_Download(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".svg") {
			w.Write([]byte("<svg></svg>"))
		} else {
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := &MockConfigProvider{Host: server.URL}
	s := New(cfg)
	s.client.Token = "test-token" // Ensure cache is clean

	homeDir, _ := os.UserHomeDir()
	cachePath := filepath.Join(homeDir, constants.AppDir, constants.CacheDir, constants.PlatformsDir, "1.svg")
	os.Remove(cachePath)
	defer os.Remove(cachePath)

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
	homeDir, _ := os.UserHomeDir()
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "snes.png") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("snes-png"))
		} else if strings.HasSuffix(r.URL.Path, "snes_alt.svg") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("snes-alt-svg"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := &MockConfigProvider{Host: server.URL}
	s := New(cfg)
	s.client.Token = "test-token"
	exts := []string{".png", ".svg"}

	// 1. Primary slug hit
	data, ext := s.tryDownloadPlatformCover("snes", exts)
	if string(data) != "snes-png" || ext != ".png" {
		t.Errorf("Expected snes-png and .png, got '%s' and '%s'", string(data), ext)
	}

	// 2. Alt slug hit
	data, ext = s.tryDownloadPlatformCover("snes-alt", exts)
	if string(data) != "snes-alt-svg" || ext != ".svg" {
		t.Errorf("Expected snes-alt-svg and .svg, got '%s' and '%s'", string(data), ext)
	}

	// 3. Miss
	data, ext = s.tryDownloadPlatformCover("nonexistent", exts)
	if data != nil || ext != "" {
		t.Errorf("Expected nil data and empty ext for download miss, got %v and %s", data, ext)
	}
}
