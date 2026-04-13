package retroarch

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go-romm-sync/constants"
	"go-romm-sync/utils/fileio"
)

// BiosInfo contains metadata about a BIOS file.
type BiosInfo struct {
	Filename  string
	Platforms []string
}

// BiosMap maps firmware MD5 hashes to BIOS metadata.
var BiosMap = map[string]BiosInfo{
	// Sega CD (Genesis Plus GX / PicoDrive)
	"baca1df271d7c11fe50087c0358f4eb5": {Filename: "bios_CD_U.bin", Platforms: []string{"segacd"}},
	"2efd74e3230dca0a2c002167732aefbe": {Filename: "bios_CD_E.bin", Platforms: []string{"segacd"}},
	"278a9397d1921baa8cf4418fb361665e": {Filename: "bios_CD_J.bin", Platforms: []string{"segacd"}},
	"854adcf5c6307185011707572793ef07": {Filename: "bios_CD_U.bin", Platforms: []string{"segacd"}}, // Model 1 V1.10
	"d266e3797c5ae401d46b7440492cb715": {Filename: "bios_CD_E.bin", Platforms: []string{"segacd"}}, // Model 1 V1.21

	// PlayStation 1 (PCSX ReARMed / Beetle PSX)
	"924e392ed05558ffdb115408c263793d": {Filename: "scph5501.bin", Platforms: []string{"ps1"}}, // US
	"6341f2aef431661646062f404439abb3": {Filename: "scph5500.bin", Platforms: []string{"ps1"}}, // JP
	"31602b9449f291076f8a846f48123cca": {Filename: "scph5502.bin", Platforms: []string{"ps1"}}, // EU
	"dc20d7c54c3031d873099ea639462d7c": {Filename: "scph1001.bin", Platforms: []string{"ps1"}},
	"0555c6ef00d1ca9e3020999827a552a0": {Filename: "scph1000.bin", Platforms: []string{"ps1"}},
	"502224b3a52f47c0e167347577e30705": {Filename: "scph7001.bin", Platforms: []string{"ps1"}},

	// PlayStation 2 (PCSX2)
	"df6f2da0472409747970973307567786": {Filename: "SCPH-39001.bin", Platforms: []string{"ps2"}},
	"9a2963162b7be533f81e6a98af074d26": {Filename: "SCPH-70012.bin", Platforms: []string{"ps2"}},

	// Game Boy Advance (mgba / gpSP)
	"a860e8c0b6d573d191e4ec7db1b1d4f6": {Filename: "gba_bios.bin", Platforms: []string{"gba"}},

	// Sega Saturn (Beetle Saturn)
	"858da2873264448576402ecb1e2a0497": {Filename: "saturn_bios.bin", Platforms: []string{"saturn"}}, // JP
	"af5828fdff5138a21dad0c9049d5668d": {Filename: "saturn_bios.bin", Platforms: []string{"saturn"}}, // US
	"294d96a0bf13e93297a72cf2626e95c1": {Filename: "saturn_bios.bin", Platforms: []string{"saturn"}}, // EU

	// Sega Dreamcast (Flycast)
	"e10c53c2f8b90bab96ead2d368858623": {Filename: "dc_bios.bin", Platforms: []string{"dreamcast"}},
	"0a93f1d8c7e35558e239c049d7494511": {Filename: "dc_flash.bin", Platforms: []string{"dreamcast"}},

	// Nintendo DS (DeSmuME / melonDS)
	"df692403c5fd37890606d51a66e409b3": {Filename: "bios7.bin", Platforms: []string{"nds"}},
	"a39217a3a2476ae067f5fa20760f3812": {Filename: "bios9.bin", Platforms: []string{"nds"}},
	"27660c6e9d67909dc75ce9100ad851f":  {Filename: "firmware.bin", Platforms: []string{"nds"}},

	// PC Engine CD / TurboGrafx-CD (Beetle PCE)
	"38179df8f4ac870017ae202c813f4cb1": {Filename: "syscard3.pce", Platforms: []string{"pce"}}, // JP
	"0757217cc277cb5624ca973b7713d2fa": {Filename: "syscard3.pce", Platforms: []string{"pce"}}, // US

	// 3DO (Opera)
	"51f2f43ae2f3508a14d9f56597e2d3ce": {Filename: "panafz10.bin", Platforms: []string{"3do"}},
	"f47264dd47fe30f73ab3c010015c155b": {Filename: "panafz1.bin", Platforms: []string{"3do"}},
	"8639fd5e549bd6238cfee79e3e749114": {Filename: "goldstar.bin", Platforms: []string{"3do"}},

	// Wonderswan (Mednafen Wonderswan)
	"f2bb97f1ccaf40f2f3611388b3941864": {Filename: "ws.rom", Platforms: []string{"wsc"}},
	"245842c15904f9dfc120a113d07e5c5f": {Filename: "wsc.rom", Platforms: []string{"wsc"}},

	// Atari 5200
	"281f20ea4320404ec820fb7ec0693b38": {Filename: "5200.rom", Platforms: []string{"a52"}},

	// Atari Lynx
	"f1870aad570baecdad59eddf14946ca3": {Filename: "lynxboot.img", Platforms: []string{"lynx"}},

	// Neo Geo Pocket / Color
	"4e1f79a299e9009951381394c8e7c10": {Filename: "npbios.bin", Platforms: []string{"ngp"}},
}

// GetBiosFilename returns the expected filename for a BIOS file given its MD5 hash.
// If the MD5 is unknown, it returns an empty string.
func GetBiosFilename(md5 string) string {
	if md5 == "" {
		return ""
	}
	if info, ok := BiosMap[strings.ToLower(md5)]; ok {
		return info.Filename
	}
	return ""
}

// GetAllBiosFilenames returns a list of all canonical BIOS filenames known to the system.
func GetAllBiosFilenames() []string {
	names := make(map[string]bool)
	for _, info := range BiosMap {
		names[info.Filename] = true
	}

	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	return result
}

// GetBiosFilenamesForPlatform returns all canonical BIOS filenames associated with a specific platform.
func GetBiosFilenamesForPlatform(platformSlug string) []string {
	names := make(map[string]bool)
	platformSlug = strings.ToLower(platformSlug)

	for _, info := range BiosMap {
		for _, p := range info.Platforms {
			if strings.EqualFold(p, platformSlug) {
				names[info.Filename] = true
				break
			}
		}
	}

	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	return result
}

type githubRelease struct {
	Assets []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func UpdateBios(ui UIProvider, exePath string) error {
	baseDir, _, err := resolveRetroArchPaths(exePath)
	if err != nil {
		return err
	}
	systemDir := GetSystemDir(baseDir)
	fileio.MkdirAll(systemDir, 0o755, ui.LogErrorf)

	ui.EventsEmit(constants.EventPlayStatus, "Fetching latest BIOS release info...")

	resp, err := http.Get(constants.URLRetroBiosLatestRelease) //nolint:bodyclose // body closed in defer
	if err != nil {
		return fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer fileio.Close(resp.Body, nil, "UpdateBios: Failed to close response body")

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to decode release info: %w", err)
	}

	downloadURL := ""
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, "RetroArch") && strings.HasSuffix(asset.Name, "_BIOS_Pack.zip") {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no suitable RetroArch BIOS pack found in the latest release")
	}

	ui.EventsEmit(constants.EventPlayStatus, "Downloading BIOS pack...")

	dlResp, err := http.Get(downloadURL) //nolint:bodyclose // body closed in defer
	if err != nil {
		return fmt.Errorf("failed to download BIOS pack: %w", err)
	}
	defer fileio.Close(dlResp.Body, nil, "UpdateBios: Failed to close download body")

	if dlResp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download BIOS pack: HTTP %d", dlResp.StatusCode)
	}

	tmpZip, err := os.CreateTemp("", "retrobios_*.zip")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer fileio.Remove(tmpZip.Name(), ui.LogErrorf)

	if _, err := io.Copy(tmpZip, dlResp.Body); err != nil {
		fileio.Close(tmpZip, ui.LogErrorf, "UpdateBios: Failed to close temp zip")
		return fmt.Errorf("failed to save BIOS pack: %w", err)
	}
	fileio.Close(tmpZip, ui.LogErrorf, "UpdateBios: Failed to close temp zip")

	ui.EventsEmit(constants.EventPlayStatus, "Extracting BIOS pack...")

	if err := unzipBios(ui, tmpZip.Name(), systemDir); err != nil {
		return fmt.Errorf("failed to extract BIOS pack: %w", err)
	}

	ui.EventsEmit(constants.EventPlayStatus, "BIOS pack updated successfully!")
	return nil
}

func unzipBios(ui UIProvider, src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer fileio.Close(r, nil, "unzipBios: Failed to close zip reader")

	var lastErr error
	errorCount := 0

	for _, f := range r.File {
		name := f.Name
		name = strings.TrimPrefix(name, "system/")

		if name == "" {
			continue
		}

		fpath := filepath.Join(dest, name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			ui.LogErrorf("illegal file path skipped: %s", fpath)
			lastErr = fmt.Errorf("illegal file path: %s", fpath)
			errorCount++
			continue
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, 0o755); err != nil {
				ui.LogErrorf("failed to create directory %s: %v", name, err)
				lastErr = err
				errorCount++
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fpath), 0o755); err != nil {
			ui.LogErrorf("failed to create directory for %s: %v", name, err)
			lastErr = err
			errorCount++
			continue
		}
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			ui.LogErrorf("failed to open output file for %s: %v", name, err)
			lastErr = err
			errorCount++
			continue
		}
		rc, err := f.Open()
		if err != nil {
			fileio.Close(outFile, nil, "unzipBios: Failed to close output file")
			ui.LogErrorf("failed to open zip member %s: %v", name, err)
			lastErr = err
			errorCount++
			continue
		}
		_, err = io.Copy(outFile, rc)
		fileio.Close(outFile, nil, "unzipBios: Failed to close output file")
		fileio.Close(rc, nil, "unzipBios: Failed to close zip member")
		if err != nil {
			ui.LogErrorf("failed to extract file %s: %v", name, err)
			lastErr = err
			errorCount++
			continue
		}
	}

	if errorCount > 0 {
		return fmt.Errorf("BIOS extraction finished with %d errors (last error: %w)", errorCount, lastErr)
	}

	return nil
}
