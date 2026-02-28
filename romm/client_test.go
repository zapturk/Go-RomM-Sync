package romm

import (
	"encoding/json"
	"go-romm-sync/types"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
		if r.FormValue("scope") != "roms.read platforms.read assets.read assets.write" {
			t.Errorf("Expected scope roms.read platforms.read assets.read assets.write, got %s", r.FormValue("scope"))
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

func TestDownloadCover(t *testing.T) {
	t.Run("internal URL - sends auth", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Errorf("Expected Authorization header")
			}
			w.Write([]byte("internal image"))
		}))
		defer server.Close()

		client := NewClient(server.URL)
		client.Token = "test-token"

		data, err := client.DownloadCover("/cover.jpg")
		if err != nil {
			t.Fatalf("DownloadCover failed: %v", err)
		}
		if string(data) != "internal image" {
			t.Errorf("Expected internal image, got %s", string(data))
		}
	})

	t.Run("external URL - no auth", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "" {
				t.Errorf("Did NOT expect Authorization header for external URL")
			}
			w.Write([]byte("external image"))
		}))
		defer server.Close()

		client := NewClient("http://romm.internal")
		client.Token = "test-token"

		data, err := client.DownloadCover(server.URL + "/cover.png")
		if err != nil || string(data) != "external image" {
			t.Errorf("External fetch failed: %v", err)
		}
	})

	t.Run("malicious subdomain - no auth", func(t *testing.T) {
		client := NewClient("http://romm.example.com")
		client.Token = "test-token"

		maliciousURL := "http://romm.example.com.attacker.com/exploit.jpg"
		if client.shouldSendToken(maliciousURL) {
			t.Errorf("shouldSendToken should be false for malicious subdomain")
		}
	})
}

func TestGetPlatforms(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id": 1, "name": "SNES", "slug": "snes"}]`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.Token = "test-token"

	platforms, err := client.GetPlatforms()
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
		w.Write([]byte(`{"id": 1, "name": "Test Game"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.Token = "test-token"

	game, err := client.GetRom(1)
	if err != nil {
		t.Fatalf("GetRom failed: %v", err)
	}
	if game.ID != 1 {
		t.Errorf("Expected ID 1, got %d", game.ID)
	}
}

func TestDownloadFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="game.sfc"`)
		w.Write([]byte("rom data"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.Token = "test-token"

	game := &types.Game{ID: 1, FullPath: "SNES/Game.sfc"}
	reader, filename, err := client.DownloadFile(game)
	if err != nil {
		t.Fatalf("DownloadFile failed: %v", err)
	}
	defer reader.Close()

	if filename != "game.sfc" {
		t.Errorf("Expected filename game.sfc, got %s", filename)
	}

	data, _ := io.ReadAll(reader)
	if string(data) != "rom data" {
		t.Errorf("Expected rom data, got %s", string(data))
	}
}

func TestUploadAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.Token = "test-token"

	err := client.UploadSave(1, "snes9x", "save.srm", []byte("save data"))
	if err != nil {
		t.Fatalf("UploadSave failed: %v", err)
	}

	err = client.UploadState(1, "snes9x", "state.st0", []byte("state data"))
	if err != nil {
		t.Fatalf("UploadState failed: %v", err)
	}
}

func TestGetSavesStates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "saves") {
			w.Write([]byte(`[{"id": 1, "filename": "save.srm"}]`))
		} else {
			w.Write([]byte(`[{"id": 1, "filename": "state.st0"}]`))
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.Token = "test-token"

	saves, err := client.GetSaves(1)
	if err != nil {
		t.Fatalf("GetSaves failed: %v", err)
	}
	if len(saves) != 1 {
		t.Errorf("Expected 1 save, got %d", len(saves))
	}

	states, err := client.GetStates(1)
	if err != nil {
		t.Fatalf("GetStates failed: %v", err)
	}
	if len(states) != 1 {
		t.Errorf("Expected 1 state, got %d", len(states))
	}
}

func TestDownloadAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="test.sav"`)
		w.Write([]byte("asset data"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.Token = "test-token"

	reader, filename, err := client.DownloadSave("/some/path/save.srm")
	if err != nil {
		t.Fatalf("DownloadSave failed: %v", err)
	}
	reader.Close()
	if filename != "test.sav" {
		t.Errorf("Expected test.sav, got %s", filename)
	}

	reader, filename, err = client.DownloadState("/some/path/state.st0")
	if err != nil {
		t.Fatalf("DownloadState failed: %v", err)
	}
	reader.Close()
	if filename != "test.sav" {
		t.Errorf("Expected test.sav, got %s", filename)
	}
}
