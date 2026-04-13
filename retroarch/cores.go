package retroarch

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"go-romm-sync/constants"
	"go-romm-sync/utils/fileio"
)

var buildbotBaseURL = "https://buildbot.libretro.com/nightly"

// ExtCoreMap maps file extensions to an ordered list of known-working libretro core
// base names. The first entry is the default used for auto-launch; subsequent entries
// are alternatives offered via the core-selector UI.
var ExtCoreMap = map[string][]string{
	// Nintendo – NES
	".nes": {"nestopia_libretro", "fceumm_libretro", "mesen_libretro"},
	".fds": {"nestopia_libretro", "fceumm_libretro"},

	// Nintendo – SNES
	".sfc": {"snes9x_libretro", "bsnes_libretro"},
	".smc": {"snes9x_libretro", "bsnes_libretro"},

	// Nintendo – N64
	".z64": {"mupen64plus_next_libretro", "parallel_n64_libretro"},
	".n64": {"mupen64plus_next_libretro", "parallel_n64_libretro"},
	".v64": {"mupen64plus_next_libretro", "parallel_n64_libretro"},

	// Nintendo – Game Boy
	".gb":  {"gambatte_libretro", "mgba_libretro", "sameboy_libretro"},
	".gbc": {"gambatte_libretro", "mgba_libretro", "sameboy_libretro"},

	// Nintendo – GBA
	".gba": {"mgba_libretro", "vba_next_libretro"},

	// Nintendo – DS
	".nds": {constants.CoreMelonDSDS, constants.CoreNooDS, constants.CoreMelonDS, constants.CoreDeSmuME},
	".dsi": {constants.CoreMelonDSDS, constants.CoreNooDS, constants.CoreMelonDS, constants.CoreDeSmuME},

	// Nintendo – Virtual Boy
	".vb": {"beetle_vb_libretro"},

	// Nintendo – GameCube / Wii
	".gcm":  {"dolphin_libretro"},
	".gcz":  {"dolphin_libretro"},
	".rvz":  {"dolphin_libretro"},
	".wbfs": {"dolphin_libretro"},
	".wia":  {"dolphin_libretro"},

	// Nintendo – 3DS
	".3ds":  {constants.CoreAzahar, constants.CoreCitra},
	".3dsx": {constants.CoreAzahar, constants.CoreCitra},
	".elf":  {constants.CoreAzahar, constants.CoreCitra},
	".axf":  {constants.CoreAzahar, constants.CoreCitra},
	".cci":  {constants.CoreAzahar, constants.CoreCitra},
	".cxi":  {constants.CoreAzahar, constants.CoreCitra},
	".app":  {constants.CoreAzahar, constants.CoreCitra},

	// Sega – Mega Drive / Genesis
	".md":  {"genesis_plus_gx_libretro", "picodrive_libretro", "blastem_libretro"},
	".smd": {"genesis_plus_gx_libretro", "picodrive_libretro"},
	".gen": {"genesis_plus_gx_libretro", "picodrive_libretro"},

	// Sega – Master System / Game Gear
	".sms": {"genesis_plus_gx_libretro", "picodrive_libretro"},
	".gg":  {"genesis_plus_gx_libretro"},

	// Sega – 32X
	".32x": {"picodrive_libretro"},

	// Sega – CD / Saturn / shared CUE / Dreamcast
	".msu": {"genesis_plus_gx_libretro"},
	".cue": {"genesis_plus_gx_libretro", "pcsx_rearmed_libretro", "mednafen_saturn_libretro", "flycast_libretro"},
	".gdi": {"flycast_libretro"},
	".cdi": {"flycast_libretro"},

	// Sony – PS1
	".bin": {"pcsx_rearmed_libretro", "beetle_psx_libretro"},

	// Sony – PSP
	".cso": {"ppsspp_libretro", "pcsx2_libretro", "play_libretro"},

	// Atari
	".a26": {"stella_libretro"},
	".a52": {"a5200_libretro"},
	".a78": {"prosystem_libretro"},
	".lnx": {"handy_libretro"},
	".jag": {"virtualjaguar_libretro"},

	// Computers
	".d64": {"vice_x64sc_libretro"},
	".prg": {"vice_x64sc_libretro"},
	".t64": {"vice_x64sc_libretro"},
	".adf": {"puae_libretro"},
	".uae": {"puae_libretro"},
	".dsk": {"caprice32_libretro", "apple2enh_libretro"},
	".sna": {"caprice32_libretro"},
	".do":  {"apple2enh_libretro"},

	// Others
	".iso": {"pcsx2_libretro", "play_libretro", "pcsx_rearmed_libretro", "beetle_psx_libretro", "opera_libretro"},
	".chd": {"pcsx2_libretro", "play_libretro", "pcsx_rearmed_libretro", "beetle_psx_libretro", "opera_libretro", "flycast_libretro"},
	".sg":  {"smsplus_libretro"},
	".col": {"gearcoleco_libretro"},
	".mx1": {"bluemsx_libretro"},
	".mx2": {"bluemsx_libretro"},
	".rom": {"bluemsx_libretro", "gearcoleco_libretro"},
	".zip": {"fbneo_libretro", "mame2003_plus_libretro"},
	".7z":  {"fbneo_libretro", "mame2003_plus_libretro"},
	".pce": {"mednafen_pce_fast_libretro", "mednafen_pce_libretro"},
	".sgx": {"mednafen_pce_fast_libretro"},
	".ws":  {"mednafen_wswan_libretro"},
	".wsc": {"mednafen_wswan_libretro"},
	".ngp": {"mednafen_ngp_libretro"},
	".ngc": {"mednafen_ngp_libretro"},

	// Pokemon Mini
	".min": {"pokemini_libretro"},

	// Pico-8
	".p8":  {"retro8_libretro"},
	".png": {constants.CoreRetro8},
}

// CoreMap is derived from ExtCoreMap for backward-compatible single-core lookups
// (used by the launcher to resolve the extension → default core).
var CoreMap = func() map[string]string {
	m := make(map[string]string, len(ExtCoreMap))
	for ext, cores := range ExtCoreMap {
		if len(cores) > 0 {
			m[ext] = cores[0]
		}
	}
	return m
}()

// GetCoresForExt returns the ordered list of known-working libretro core base-names
// for the given file extension (e.g. ".gb"). The first entry is the default.
// Returns nil if no cores are known for the extension.
func GetCoresForExt(ext string) []string {
	return ExtCoreMap[strings.ToLower(ext)]
}

// GetCoresFromZip peeks inside a ZIP file and returns a combined list of cores
// for all recognized file extensions found inside.
func GetCoresFromZip(zipPath string) []string {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil
	}
	defer fileio.Close(r, nil, "GetCoresFromZip: Failed to close zip reader")

	var cores []string
	seen := make(map[string]bool)

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		innerExt := strings.ToLower(filepath.Ext(f.Name))
		if innerCores := GetCoresForExt(innerExt); len(innerCores) > 0 {
			for _, core := range innerCores {
				if !seen[core] {
					cores = append(cores, core)
					seen[core] = true
				}
			}
		}
	}
	return cores
}

// DownloadCore fetches a missing core from Libretro buildbot
func DownloadCore(ui UIProvider, coreFile, coresDir, arch string) error {
	ui.EventsEmit(constants.EventPlayStatus, fmt.Sprintf("Downloading missing core: %s...", coreFile))

	var osName, archName string
	switch runtime.GOOS {
	case constants.OSWindows:
		osName = constants.OSWindows
	case constants.OSDarwin:
		osName = "apple/osx"
	case constants.OSLinux:
		osName = constants.OSLinux
	default:
		return fmt.Errorf("unsupported OS for core downloads: %s", runtime.GOOS)
	}

	switch arch {
	case constants.ArchAmd64:
		archName = "x86_64"
	case constants.ArchArm64:
		if runtime.GOOS == constants.OSDarwin {
			archName = constants.ArchArm64
		} else {
			archName = "aarch64"
		}
	case constants.Arch386:
		archName = "x86"
	default:
		return fmt.Errorf("unsupported arch for core downloads: %s", arch)
	}

	urlStr := fmt.Sprintf("%s/%s/%s/latest/%s.zip", buildbotBaseURL, osName, archName, coreFile)

	resp, err := http.Get(urlStr) //nolint:bodyclose // body is closed via fileio.Close wrapper below
	if err != nil {
		return fmt.Errorf("failed to download core: %w", err)
	}
	defer fileio.Close(resp.Body, nil, "DownloadCore: Failed to close response body")

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("core download failed with status 404 from %s", urlStr)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("core download failed with status %d from %s", resp.StatusCode, urlStr)
	}

	fileio.MkdirAll(coresDir, 0o755, ui.LogErrorf)
	zipPath := filepath.Join(coresDir, coreFile+".zip")
	out, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("failed to create core zip: %w", err)
	}
	_, err = io.Copy(out, resp.Body)
	fileio.Close(out, nil, "DownloadCore: Failed to close core zip file")
	if err != nil {
		return fmt.Errorf("failed to save core zip: %w", err)
	}
	defer fileio.Remove(zipPath, ui.LogErrorf)

	err = unzipCore(zipPath, coresDir)
	if err != nil {
		return fmt.Errorf("failed to extract core: %w", err)
	}

	ui.EventsEmit(constants.EventPlayStatus, "Core downloaded successfully!")
	return nil
}

// unzipCore extracts a standard zip archive into a destination directory
func unzipCore(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer fileio.Close(r, nil, "unzipCore: Failed to close zip reader")

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}
		if f.FileInfo().IsDir() {
			fileio.MkdirAll(fpath, 0o755, nil)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fpath), 0o755); err != nil {
			return err
		}
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			fileio.Close(outFile, nil, "unzipCore: Failed to close output file")
			return err
		}
		_, err = io.Copy(outFile, rc)
		fileio.Close(outFile, nil, "unzipCore: Failed to close output file")
		fileio.Close(rc, nil, "unzipCore: Failed to close zip member")
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateAllCores scans the local cores directory and re-downloads all existing cores
// to ensure they are up-to-date.
func UpdateAllCores(ui UIProvider, exePath string) error {
	baseDir, binaryPath, err := resolveRetroArchPaths(exePath)
	if err != nil {
		return err
	}
	coresDir := getCoresDir(baseDir)

	entries, err := os.ReadDir(coresDir)
	if err != nil {
		if os.IsNotExist(err) {
			ui.EventsEmit(constants.EventPlayStatus, "No cores found to update.")
			return nil
		}
		return fmt.Errorf("failed to read cores directory: %w", err)
	}

	arch := detectRetroArchArch(ui, binaryPath)
	var updatedCount atomic.Int32
	var wg sync.WaitGroup

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".dll" || ext == ".so" || ext == ".dylib" {
			coreFile := entry.Name()
			wg.Add(1)
			go func(cf string) {
				defer wg.Done()
				ui.LogInfof("Updating core: %s", cf)
				err := DownloadCore(ui, cf, coresDir, arch)
				if err != nil {
					ui.LogErrorf("Failed to update core %s: %v", cf, err)
				} else {
					updatedCount.Add(1)
				}
			}(coreFile)
		}
	}

	wg.Wait()
	ui.EventsEmit(constants.EventPlayStatus, fmt.Sprintf("Finished updating %d cores.", updatedCount.Load()))
	return nil
}
