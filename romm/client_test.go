package romm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/token" {
			t.Errorf("Expected path /api/token, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("Expected method POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Expected Content-Type application/x-www-form-urlencoded, got %s", r.Header.Get("Content-Type"))
		}

		if err := r.ParseForm(); err != nil {
			t.Errorf("Failed to parse form: %v", err)
		}
		if r.FormValue("scope") != "roms.read platforms.read" {
			t.Errorf("Expected scope roms.read platforms.read, got %s", r.FormValue("scope"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": "test-token",
			"token_type":   "bearer",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	token, err := client.Login("user", "pass")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if token != "test-token" {
		t.Errorf("Expected token test-token, got %s", token)
	}
}

func TestGetLibrary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/roms" {
			t.Errorf("Expected path /api/roms, got %s", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("Expected method GET, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Expected Authorization header Bearer test-token, got %s", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		// Respond with a paginated list of games
		w.Write([]byte(`{"items": [{"id": 1, "name": "Test Game", "rom_id": 123, "url_cover": "http://example.com/cover.jpg"}], "total": 1}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.Token = "test-token"

	games, err := client.GetLibrary()
	if err != nil {
		t.Fatalf("GetLibrary failed: %v", err)
	}

	if len(games) != 1 {
		t.Errorf("Expected 1 game, got %d", len(games))
	}
	if games[0].Title != "Test Game" {
		t.Errorf("Expected game title Test Game, got %s", games[0].Title)
	}
}
