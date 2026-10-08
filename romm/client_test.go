package romm

import (
	"context"
	"encoding/json"
	"go-romm-sync/constants"
	"go-romm-sync/types"
	"io"
	"net"
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
		if r.FormValue("scope") != constants.RomMLoginScopes {
			t.Errorf("Expected scope %s, got %s", constants.RomMLoginScopes, r.FormValue("scope"))
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
	t.Run("base case", func(t *testing.T) {
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
			if r.URL.Query().Get("platform_ids") != "1" {
				t.Errorf("Expected platform_ids=1, got %s", r.URL.Query().Get("platform_ids"))
			}
			if r.URL.Query().Get("limit") != "30" {
				t.Errorf("Expected limit=30, got %s", r.URL.Query().Get("limit"))
			}

			w.Header().Set("Content-Type", "application/json")
			// Respond with a paginated list of games
			w.Write([]byte(`{"items": [{"id": 1, "name": "Test Game", "rom_id": 123, "url_cover": "http://example.com/cover.jpg"}], "total_count": 1}`))
		}))
		defer server.Close()

		client := NewClient(server.URL)
		client.Token = "test-token"

		games, _, err := client.GetLibrary(30, 0, 1, "")
		if err != nil {
			t.Fatalf("GetLibrary failed: %v", err)
		}

		if len(games) != 1 {
			t.Errorf("Expected 1 game, got %d", len(games))
		}
		if games[0].Title != "Test Game" {
			t.Errorf("Expected game title Test Game, got %s", games[0].Title)
		}
	})

	t.Run("with search term", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("search_term") != "zelda" {
				t.Errorf("Expected search_term=zelda, got %s", r.URL.Query().Get("search_term"))
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"items": [], "total_count": 0}`))
		}))
		defer server.Close()

		client := NewClient(server.URL)
		client.Token = "test-token"

		_, _, err := client.GetLibrary(30, 0, 0, "zelda")
		if err != nil {
			t.Fatalf("GetLibrary with search failed: %v", err)
		}
	})
}

func TestDownloadCover(t *testing.T) {
	// customDialer resolves test hostnames (romm.local, cdn.local) to 127.0.0.1
	customDialer := func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, p, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if host == "romm.local" || host == "cdn.local" {
			host = "127.0.0.1"
		}
		return net.Dial("tcp", net.JoinHostPort(host, p))
	}

	t.Run("internal URL - sends auth", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Errorf("Expected Authorization header")
			}
			w.Write([]byte("internal image"))
		}))
		defer server.Close()

		port := strings.TrimPrefix(server.URL, "http://127.0.0.1:")

		client := NewClient("http://romm.local:" + port)
		client.Token = "test-token"
		client.FileClient = &http.Client{Transport: &http.Transport{DialContext: customDialer}}

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

		port := strings.TrimPrefix(server.URL, "http://127.0.0.1:")

		client := NewClient("http://romm.local")
		client.Token = "test-token"
		client.FileClient = &http.Client{Transport: &http.Transport{DialContext: customDialer}}

		data, err := client.DownloadCover("http://cdn.local:" + port + "/cover.png")
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
	t.Run("array response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("limit") != "50" || r.URL.Query().Get("offset") != "10" {
				t.Errorf("Expected limit=50, offset=10, got limit=%s, offset=%s", r.URL.Query().Get("limit"), r.URL.Query().Get("offset"))
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[{"id": 1, "name": "SNES", "slug": "snes", "rom_count": 1}]`))
		}))
		defer server.Close()

		client := NewClient(server.URL)
		client.Token = "test-token"
		platforms, total, err := client.GetPlatforms(50, 10)
		if err != nil {
			t.Fatalf("GetPlatforms failed: %v", err)
		}
		if len(platforms) != 1 || total != 1 {
			t.Errorf("Expected 1 platform, total 1; got len=%d, total=%d", len(platforms), total)
		}
	})

	t.Run("paginated object response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"items": [{"id": 1, "name": "NES", "slug": "nes", "rom_count": 5}], "total_count": 100}`))
		}))
		defer server.Close()

		client := NewClient(server.URL)
		client.Token = "test-token"
		platforms, total, err := client.GetPlatforms(30, 0)
		if err != nil {
			t.Fatalf("GetPlatforms failed: %v", err)
		}
		if len(platforms) != 1 || total != 100 {
			t.Errorf("Expected 1 platform, total 100; got len=%d, total=%d", len(platforms), total)
		}
		if platforms[0].Name != "NES" {
			t.Errorf("Expected NES, got %s", platforms[0].Name)
		}
	})
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
	reader, filename, err := client.DownloadFile(context.Background(), game)
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

	err := client.UploadSave(1, "snes9x", "save.srm", []byte("save data"), "default")
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

	// Test DownloadSave
	reader, filename, err := client.DownloadSave(context.Background(), 1)
	if err != nil {
		t.Fatalf("DownloadSave failed: %v", err)
	}
	reader.Close()
	if filename != "test.sav" {
		t.Errorf("Expected test.sav, got %s", filename)
	}

	// Test DownloadState
	reader, filename, err = client.DownloadState(context.Background(), 2)
	if err != nil {
		t.Fatalf("DownloadState failed: %v", err)
	}
	reader.Close()
	if filename != "test.sav" {
		t.Errorf("Expected test.sav, got %s", filename)
	}
}
func TestReadAllWithLimit(t *testing.T) {
	client := NewClient("http://localhost")
	limit := int64(10)

	t.Run("under limit", func(t *testing.T) {
		r := strings.NewReader("hello")
		data, err := client.readAllWithLimit(r, limit)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if string(data) != "hello" {
			t.Errorf("Expected hello, got %s", string(data))
		}
	})

	t.Run("at limit", func(t *testing.T) {
		r := strings.NewReader("0123456789")
		data, err := client.readAllWithLimit(r, limit)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if len(data) != 10 {
			t.Errorf("Expected 10 bytes, got %d", len(data))
		}
	})

	t.Run("over limit", func(t *testing.T) {
		r := strings.NewReader("0123456789A")
		data, err := client.readAllWithLimit(r, limit)
		if err == nil {
			t.Error("Expected error for exceeding limit, got nil")
		}
		if !strings.Contains(err.Error(), "exceeded limit") {
			t.Errorf("Expected limit exceeded error, got: %v", err)
		}
		if len(data) != 10 {
			t.Errorf("Expected 10 bytes (truncated), got %d", len(data))
		}
	})
}

func TestGetPlatform(t *testing.T) {
	t.Run("no custom name", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id": 42, "name": "SNES", "custom_name": "", "slug": "snes"}`))
		}))
		defer server.Close()

		client := NewClient(server.URL)
		client.Token = "test-token"

		platform, err := client.GetPlatform(42)
		if err != nil {
			t.Fatalf("GetPlatform failed: %v", err)
		}
		if platform.Name != "SNES" {
			t.Errorf("Expected Name 'SNES', got '%s'", platform.Name)
		}
	})

	t.Run("with custom name", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id": 42, "name": "SNES", "custom_name": "My Custom SNES", "slug": "snes"}`))
		}))
		defer server.Close()

		client := NewClient(server.URL)
		client.Token = "test-token"

		platform, err := client.GetPlatform(42)
		if err != nil {
			t.Fatalf("GetPlatform failed: %v", err)
		}
		if platform.Name != "My Custom SNES" {
			t.Errorf("Expected Name 'My Custom SNES', got '%s'", platform.Name)
		}
		if platform.CustomName != "My Custom SNES" {
			t.Errorf("Expected CustomName 'My Custom SNES', got '%s'", platform.CustomName)
		}
	})
}

func TestDownloadRomFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/roms/101/files/content/disc1.bin" {
				t.Errorf("Expected path /api/roms/101/files/content/disc1.bin, got %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Errorf("Expected Authorization header Bearer test-token, got %s", r.Header.Get("Authorization"))
			}
			w.Write([]byte("fake-bin-data"))
		}))
		defer server.Close()

		client := NewClient(server.URL)
		client.Token = "test-token"

		reader, err := client.DownloadRomFile(context.Background(), 101, "disc1.bin")
		if err != nil {
			t.Fatalf("DownloadRomFile failed: %v", err)
		}
		defer reader.Close()

		content, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("Failed to read body: %v", err)
		}
		if string(content) != "fake-bin-data" {
			t.Errorf("Expected content 'fake-bin-data', got '%s'", string(content))
		}
	})

	t.Run("unauthenticated", func(t *testing.T) {
		client := NewClient("http://localhost")
		_, err := client.DownloadRomFile(context.Background(), 101, "disc1.bin")
		if err == nil {
			t.Error("Expected error for unauthenticated client, got nil")
		}
	})
}

func TestUploadSave_Slot(t *testing.T) {
	var capturedSlot string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSlot = r.URL.Query().Get("slot")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.Token = "test-token"

	err := client.UploadSave(1, "snes9x", "save.srm", []byte("data"), "custom_slot")
	if err != nil {
		t.Fatalf("UploadSave failed: %v", err)
	}
	if capturedSlot != "custom_slot" {
		t.Errorf("Expected slot 'custom_slot', got '%s'", capturedSlot)
	}
}

func TestGetSavesForSlot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		slotQuery := r.URL.Query().Get("slot")
		if slotQuery == "slotA" {
			w.Write([]byte(`[{"id": 1, "filename": "saveA.srm", "slot": "slotA"}]`))
		} else {
			w.Write([]byte(`[
				{"id": 1, "filename": "saveA.srm", "slot": "slotA"},
				{"id": 2, "filename": "legacy.srm", "slot": null}
			]`))
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.Token = "test-token"

	// Named slot
	savesA, err := client.GetSavesForSlot(1, "slotA")
	if err != nil {
		t.Fatalf("GetSavesForSlot failed: %v", err)
	}
	if len(savesA) != 1 || savesA[0].Slot != "slotA" {
		t.Errorf("Expected 1 save in slotA, got %v", savesA)
	}

	// Legacy slot (empty string)
	legacySaves, err := client.GetSavesForSlot(1, "")
	if err != nil {
		t.Fatalf("GetSavesForSlot legacy failed: %v", err)
	}
	if len(legacySaves) != 1 || legacySaves[0].Slot != "" {
		t.Errorf("Expected 1 legacy save, got %v", legacySaves)
	}
}

func TestGetSavesForSlot_ReturnsMostRecentSave(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[
			{"id": 1, "filename": "metroid [2026-10-01_10-00-00].srm", "slot": "speedrun", "updated_at": "2026-10-01T10:00:00Z"},
			{"id": 5, "filename": "metroid [2026-10-07_12-00-00].srm", "slot": "speedrun", "updated_at": "2026-10-07T12:00:00Z"},
			{"id": 3, "filename": "metroid [2026-10-05_09-00-00].srm", "slot": "speedrun", "updated_at": "2026-10-05T09:00:00Z"},
			{"id": 2, "filename": "metroid.rtc", "slot": "speedrun", "updated_at": "2026-10-06T11:00:00Z"}
		]`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.Token = "test-token"

	saves, err := client.GetSavesForSlot(1, "speedrun")
	if err != nil {
		t.Fatalf("GetSavesForSlot failed: %v", err)
	}

	if len(saves) != 2 {
		t.Fatalf("Expected 2 saves, got %d: %v", len(saves), saves)
	}

	if saves[0].ID != 5 {
		t.Errorf("Expected newest save id 5 first, got id %d", saves[0].ID)
	}
	if CleanSaveFileName(saves[0].GetEffectiveFileName()) != "metroid.srm" {
		t.Errorf("Expected clean filename metroid.srm, got %s", CleanSaveFileName(saves[0].GetEffectiveFileName()))
	}

	if saves[1].ID != 2 {
		t.Errorf("Expected second save id 2, got id %d", saves[1].ID)
	}
}

func TestGetSaveSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/saves/summary" {
			w.Write([]byte(`{
				"total_count": 2,
				"slots": [
					{"slot": "default", "count": 1, "latest": {"id": 1, "updated_at": "2026-04-10T10:00:00Z"}},
					{"slot": null, "count": 1, "latest": {"id": 2, "updated_at": "2026-04-09T10:00:00Z"}}
				]
			}`))
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.Token = "test-token"

	summary, err := client.GetSaveSummary(1)
	if err != nil {
		t.Fatalf("GetSaveSummary failed: %v", err)
	}
	if summary.TotalCount != 2 || len(summary.Slots) != 2 {
		t.Errorf("Expected 2 slots, got %v", summary)
	}
}

func TestDeleteServerSaves(t *testing.T) {
	var receivedBody map[string][]uint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/saves/delete" || r.Method != "POST" {
			http.NotFound(w, r)
			return
		}
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.Token = "test-token"

	err := client.DeleteServerSaves([]uint{10, 20})
	if err != nil {
		t.Fatalf("DeleteServerSaves failed: %v", err)
	}
	if len(receivedBody["saves"]) != 2 || receivedBody["saves"][0] != 10 {
		t.Errorf("Expected received saves [10, 20], got %v", receivedBody)
	}
}
