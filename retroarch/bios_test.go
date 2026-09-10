package retroarch

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"go-romm-sync/constants"
)

func TestFindBiosAssets_MultiPart(t *testing.T) {
	release := &githubRelease{
		Assets: []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
		}{
			{Name: "Batocera_43.1_BIOS_Pack.zip.001", BrowserDownloadURL: "https://example.com/batocera.001", Size: 100},
			{Name: "Batocera_43.1_BIOS_Pack.zip.002", BrowserDownloadURL: "https://example.com/batocera.002", Size: 100},
			{Name: "RetroArch_Lakka_v1.22.2_BIOS_Pack.zip.002", BrowserDownloadURL: "https://example.com/ra.002", Size: 1357136371},
			{Name: "RetroArch_Lakka_v1.22.2_BIOS_Pack.zip.001", BrowserDownloadURL: "https://example.com/ra.001", Size: 1992294400},
			{Name: "RetroBat_8.2.1_BIOS_Pack.zip.001", BrowserDownloadURL: "https://example.com/retrobat.001", Size: 100},
			{Name: "RomM_5.2.0_BIOS_Pack.zip", BrowserDownloadURL: "https://example.com/romm.zip", Size: 100},
			{Name: "SHA256SUMS.txt", BrowserDownloadURL: "https://example.com/sums.txt", Size: 50},
		},
	}

	parts, err := findBiosAssets(release)
	if err != nil {
		t.Fatalf("findBiosAssets failed: %v", err)
	}

	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}

	if parts[0].Name != "RetroArch_Lakka_v1.22.2_BIOS_Pack.zip.001" || parts[0].PartNumber != 1 {
		t.Errorf("expected part 1 first, got %+v", parts[0])
	}
	if parts[0].BrowserDownloadURL != "https://example.com/ra.001" {
		t.Errorf("unexpected part 1 URL: %s", parts[0].BrowserDownloadURL)
	}

	if parts[1].Name != "RetroArch_Lakka_v1.22.2_BIOS_Pack.zip.002" || parts[1].PartNumber != 2 {
		t.Errorf("expected part 2 second, got %+v", parts[1])
	}
	if parts[1].BrowserDownloadURL != "https://example.com/ra.002" {
		t.Errorf("unexpected part 2 URL: %s", parts[1].BrowserDownloadURL)
	}
}

func TestFindBiosAssets_SingleZip(t *testing.T) {
	release := &githubRelease{
		Assets: []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
		}{
			{Name: "BizHawk_2.11.1_BIOS_Pack.zip", BrowserDownloadURL: "https://example.com/bizhawk.zip", Size: 100},
			{Name: "RetroArch_v1.20.0_BIOS_Pack.zip", BrowserDownloadURL: "https://example.com/ra_single.zip", Size: 500},
		},
	}

	parts, err := findBiosAssets(release)
	if err != nil {
		t.Fatalf("findBiosAssets failed: %v", err)
	}

	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0].Name != "RetroArch_v1.20.0_BIOS_Pack.zip" {
		t.Errorf("expected RetroArch_v1.20.0_BIOS_Pack.zip, got %s", parts[0].Name)
	}
	if parts[0].PartNumber != 1 {
		t.Errorf("expected PartNumber 1, got %d", parts[0].PartNumber)
	}
}

func TestFindBiosAssets_MissingPart(t *testing.T) {
	// Missing part 2
	release := &githubRelease{
		Assets: []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
		}{
			{Name: "RetroArch_Lakka_v1.22.2_BIOS_Pack.zip.001", BrowserDownloadURL: "https://example.com/ra.001", Size: 100},
			{Name: "RetroArch_Lakka_v1.22.2_BIOS_Pack.zip.003", BrowserDownloadURL: "https://example.com/ra.003", Size: 100},
		},
	}

	_, err := findBiosAssets(release)
	if err == nil {
		t.Fatal("expected error for non-contiguous split parts")
	}
}

func TestFindBiosAssets_MissingPart1(t *testing.T) {
	// Only part 2 present
	release := &githubRelease{
		Assets: []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
		}{
			{Name: "RetroArch_Lakka_v1.22.2_BIOS_Pack.zip.002", BrowserDownloadURL: "https://example.com/ra.002", Size: 100},
		},
	}

	_, err := findBiosAssets(release)
	if err == nil {
		t.Fatal("expected error when part 1 is missing")
	}
}

func TestFindBiosAssets_NoMatch(t *testing.T) {
	release := &githubRelease{
		Assets: []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
		}{
			{Name: "RetroPie_v1.22.2_BIOS_Pack.zip.001", BrowserDownloadURL: "https://example.com/retropie.001", Size: 100},
			{Name: "RetroBat_8.2.1_BIOS_Pack.zip.001", BrowserDownloadURL: "https://example.com/retrobat.001", Size: 100},
		},
	}

	_, err := findBiosAssets(release)
	if err == nil {
		t.Fatal("expected error when no RetroArch BIOS pack is present")
	}
}

func createTestBiosZip() (part1, part2 []byte, err error) {
	zipBuf := new(bytes.Buffer)
	zw := zip.NewWriter(zipBuf)

	files := map[string]string{
		"scph1001.bin":        "playstation-bios-data",
		"system/gba_bios.bin": "gameboy-advance-bios-data",
		"dc/dc_boot.bin":      "dreamcast-boot-data",
	}
	for name, content := range files {
		f, createErr := zw.Create(name)
		if createErr != nil {
			return nil, nil, createErr
		}
		if _, writeErr := f.Write([]byte(content)); writeErr != nil {
			return nil, nil, writeErr
		}
	}

	if closeErr := zw.Close(); closeErr != nil {
		return nil, nil, closeErr
	}

	zipBytes := zipBuf.Bytes()
	splitPoint := len(zipBytes) / 2
	return zipBytes[:splitPoint], zipBytes[splitPoint:], nil
}

func verifyExtractedBios(t *testing.T, tempDir string) {
	expected := map[string]string{
		filepath.Join(tempDir, "scph1001.bin"):      "playstation-bios-data",
		filepath.Join(tempDir, "gba_bios.bin"):      "gameboy-advance-bios-data",
		filepath.Join(tempDir, "dc", "dc_boot.bin"): "dreamcast-boot-data",
	}
	for path, expectedContent := range expected {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s not extracted: %v", filepath.Base(path), err)
		}
		if string(content) != expectedContent {
			t.Errorf("unexpected content for %s: %s", filepath.Base(path), string(content))
		}
	}
}

func TestUpdateBios_MultiPart_Integration(t *testing.T) {
	part1Bytes, part2Bytes, err := createTestBiosZip()
	if err != nil {
		t.Fatalf("failed to build test zip: %v", err)
	}

	// Set up mock HTTP server
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest":
			rel := map[string]interface{}{
				"assets": []map[string]interface{}{
					{
						"name":                 "RetroArch_Lakka_v1.22.2_BIOS_Pack.zip.001",
						"browser_download_url": server.URL + "/download/ra.zip.001",
						"size":                 len(part1Bytes),
					},
					{
						"name":                 "RetroArch_Lakka_v1.22.2_BIOS_Pack.zip.002",
						"browser_download_url": server.URL + "/download/ra.zip.002",
						"size":                 len(part2Bytes),
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(rel)
		case "/download/ra.zip.001":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(part1Bytes)
		case "/download/ra.zip.002":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(part2Bytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldURL := biosReleaseURL
	biosReleaseURL = server.URL + "/releases/latest"
	defer func() { biosReleaseURL = oldURL }()

	tempDir, err := os.MkdirTemp("", "test_system_dir_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	oldSysDir := overrideSystemDir
	overrideSystemDir = tempDir
	defer func() { overrideSystemDir = oldSysDir }()

	dummyExe := filepath.Join(tempDir, "retroarch")
	if err := os.WriteFile(dummyExe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("failed to create dummy exe: %v", err)
	}

	ui := &MockUI{}

	if err := UpdateBios(ui, dummyExe); err != nil {
		t.Fatalf("UpdateBios failed: %v", err)
	}

	verifyExtractedBios(t, tempDir)

	// 6. Verify UI events
	if ui.EmittedEvents[constants.EventPlayStatus] == 0 {
		t.Errorf("expected EventPlayStatus events, got 0")
	}

	foundSuccess := false
	for _, msg := range ui.EmittedEventMsgs[constants.EventPlayStatus] {
		if msg == "BIOS pack updated successfully!" {
			foundSuccess = true
			break
		}
	}
	if !foundSuccess {
		t.Errorf("expected 'BIOS pack updated successfully!' status message, got: %v", ui.EmittedEventMsgs[constants.EventPlayStatus])
	}

	if ui.EmittedEvents["bios-download-progress"] == 0 {
		t.Errorf("expected bios-download-progress events, got 0")
	}
}

func TestProgressWriter(t *testing.T) {
	ui := &MockUI{}
	pw := &progressWriter{
		total: 1000,
		ui:    ui,
	}

	buf := make([]byte, 250)
	_, _ = pw.Write(buf)
	if pw.lastPercent != 25 {
		t.Errorf("expected 25%%, got %d%%", pw.lastPercent)
	}

	_, _ = pw.Write(buf)
	if pw.lastPercent != 50 {
		t.Errorf("expected 50%%, got %d%%", pw.lastPercent)
	}

	buf500 := make([]byte, 500)
	_, _ = pw.Write(buf500)
	if pw.lastPercent != 100 {
		t.Errorf("expected 100%%, got %d%%", pw.lastPercent)
	}

	// Exceed total size should cap at 100
	_, _ = pw.Write(buf)
	if pw.lastPercent != 100 {
		t.Errorf("expected 100%% capped, got %d%%", pw.lastPercent)
	}
}

func TestFindBiosAssets_LiveRelease(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live network test in short mode")
	}

	resp, err := http.Get(constants.URLRetroBiosLatestRelease)
	if err != nil {
		t.Skipf("skipping live network test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Skipf("skipping live network test: HTTP %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		t.Fatalf("failed to decode live release: %v", err)
	}

	parts, err := findBiosAssets(&release)
	if err != nil {
		t.Fatalf("findBiosAssets failed on live release: %v", err)
	}

	if len(parts) == 0 {
		t.Fatal("expected at least 1 part from live release")
	}

	t.Logf("Found %d parts from live release:", len(parts))
	for _, p := range parts {
		t.Logf("  Part %d: %s (%d bytes)", p.PartNumber, p.Name, p.Size)
		if p.BrowserDownloadURL == "" {
			t.Errorf("empty download URL for part %d", p.PartNumber)
		}
	}
}
