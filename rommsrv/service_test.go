package rommsrv

import (
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

	games, err := s.GetLibrary()
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

	platforms, err := s.GetPlatforms()
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

// Note: TestGetCover and TestGetPlatformCover would ideally mock the client's DownloadCover method.
// Since romm.Client's methods are not on an interface here, it's harder to mock without changing the design or using a test server.
// However, we can test the local file stat/read logic if we setup a dummy file.

func TestGetCover_Cached(t *testing.T) {
	homeDir, _ := os.UserHomeDir()
	cacheDir := filepath.Join(homeDir, ".go-romm-sync", "cache", "covers")
	os.MkdirAll(cacheDir, 0755)

	romID := uint(9999)
	cachePath := filepath.Join(cacheDir, "9999.jpg")
	os.WriteFile(cachePath, []byte("dummy image data"), 0644)
	defer os.Remove(cachePath)

	s := &Service{} // client not needed for cached path
	data, err := s.GetCover(romID, "http://example.com/cover.jpg")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if data == "" {
		t.Errorf("Expected base64 data, got empty string")
	}
}

func TestGetCover_Download(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("downloaded image data"))
	}))
	defer server.Close()

	cfg := &MockConfigProvider{Host: server.URL}
	s := New(cfg)
	s.client.Token = "test-token"
	// No need to manually set client.BaseURL as New(cfg) does it.

	// Ensure cache is clean for this ID
	homeDir, _ := os.UserHomeDir()
	cachePath := filepath.Join(homeDir, ".go-romm-sync", "cache", "covers", "1234.jpg")
	os.Remove(cachePath)
	defer os.Remove(cachePath)

	data, err := s.GetCover(1234, "/some/cover.jpg")
	if err != nil {
		t.Fatalf("GetCover failed: %v", err)
	}
	if data == "" {
		t.Errorf("Expected base64 data")
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
	s.client.Token = "test-token"

	// Ensure cache is clean
	homeDir, _ := os.UserHomeDir()
	cachePath := filepath.Join(homeDir, ".go-romm-sync", "cache", "platforms", "1.svg")
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
