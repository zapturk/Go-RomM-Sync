package rommsrv

import (
	"encoding/json"
	"fmt"
	"go-romm-sync/types"
	"net/http"
	"net/http/httptest"
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

func (m *MockConfigProvider) GetClientToken() string {
	return ""
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

	games, _, err := s.GetLibrary(30, 0, 1, "")
	if err != nil {
		t.Fatalf("GetLibrary failed: %v", err)
	}
	if len(games) != 1 {
		t.Errorf("Expected 1 game, got %d", len(games))
	}
}

func TestGetPlatforms(t *testing.T) {
	// Mock server that returns platforms in batches
	platformsData := []types.Platform{
		{ID: 1, Name: "SNES", Slug: "snes", RomCount: 1},       // Supported 1
		{ID: 2, Name: "NES", Slug: "nes", RomCount: 1},         // Supported 2
		{ID: 3, Name: "Unknown", Slug: "unknown", RomCount: 1}, // Unsupported
		{ID: 4, Name: "GB", Slug: "gb", RomCount: 0},           // Supported but 0 roms (filtered)
		{ID: 5, Name: "GBA", Slug: "gba", RomCount: 1},         // Supported 3
		{ID: 6, Name: "Genesis", Slug: "genesis", RomCount: 1}, // Supported 4
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limitStr := r.URL.Query().Get("limit")
		offsetStr := r.URL.Query().Get("offset")
		var limit, offset int
		fmt.Sscanf(limitStr, "%d", &limit)
		fmt.Sscanf(offsetStr, "%d", &offset)

		end := offset + limit
		if end > len(platformsData) {
			end = len(platformsData)
		}

		batch := []types.Platform{}
		if offset < len(platformsData) {
			batch = platformsData[offset:end]
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items":       batch,
			"total_count": len(platformsData),
		})
	}))
	defer server.Close()

	cfg := &MockConfigProvider{Host: server.URL}
	s := New(cfg)
	s.client.Token = "test-token"

	t.Run("first page", func(t *testing.T) {
		platforms, total, err := s.GetPlatforms(2, 0)
		if err != nil {
			t.Fatalf("GetPlatforms failed: %v", err)
		}
		// Supported found: SNES, NES, GBA, Genesis (Total 4)
		// Limit 2, Offset 0: Returns SNES, NES
		if len(platforms) != 2 {
			t.Errorf("Expected 2 platforms, got %d", len(platforms))
		}
		if total != 4 {
			t.Errorf("Expected total 4 supported platforms, got %d", total)
		}
		if platforms[0].Slug != "snes" || platforms[1].Slug != "nes" {
			t.Errorf("Unexpected platforms: %s, %s", platforms[0].Slug, platforms[1].Slug)
		}
	})

	t.Run("second page", func(t *testing.T) {
		platforms, total, err := s.GetPlatforms(2, 2)
		if err != nil {
			t.Fatalf("GetPlatforms failed: %v", err)
		}
		// Limit 2, Offset 2: Returns GBA, Genesis
		if len(platforms) != 2 {
			t.Errorf("Expected 2 platforms, got %d", len(platforms))
		}
		if total != 4 {
			t.Errorf("Expected total 4 supported platforms, got %d", total)
		}
		if platforms[0].Slug != "gba" || platforms[1].Slug != "genesis" {
			t.Errorf("Unexpected platforms: %s, %s", platforms[0].Slug, platforms[1].Slug)
		}
	})
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


// Tests moved to assets package
