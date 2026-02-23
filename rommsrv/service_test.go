package rommsrv

import (
	"os"
	"path/filepath"
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
