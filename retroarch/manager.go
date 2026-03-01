package retroarch

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"go-romm-sync/constants"
	"go-romm-sync/utils/fileio"
)

// UIProvider defines the UI and logging interactions needed for RetroArch.
type UIProvider interface {
	LogInfof(format string, args ...interface{})
	LogErrorf(format string, args ...interface{})
	EventsEmit(eventName string, args ...interface{})
	WindowHide()
	WindowShow()
	WindowUnminimise()
}

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
	".nds": {"melonds_libretro", "desmume_libretro"},
	".dsi": {"melonds_libretro", "desmume_libretro"},

	// Nintendo – Virtual Boy
	".vb": {"beetle_vb_libretro"},

	// Nintendo – GameCube / Wii
	".gcm":  {"dolphin_libretro"},
	".gcz":  {"dolphin_libretro"},
	".rvz":  {"dolphin_libretro"},
	".wbfs": {"dolphin_libretro"},
	".wia":  {"dolphin_libretro"},

	// Nintendo – 3DS
	".3ds":  {constants.CoreCitra},
	".3dsx": {constants.CoreCitra},
	".elf":  {constants.CoreCitra},
	".axf":  {constants.CoreCitra},
	".cci":  {constants.CoreCitra},
	".cxi":  {constants.CoreCitra},
	".app":  {constants.CoreCitra},

	// Sega – Mega Drive / Genesis
	".md":  {"genesis_plus_gx_libretro", "picodrive_libretro", "blastem_libretro"},
	".smd": {"genesis_plus_gx_libretro", "picodrive_libretro"},
	".gen": {"genesis_plus_gx_libretro", "picodrive_libretro"},

	// Sega – Master System / Game Gear
	".sms": {"genesis_plus_gx_libretro", "picodrive_libretro"},
	".gg":  {"genesis_plus_gx_libretro"},

	// Sega – 32X
	".32x": {"picodrive_libretro"},

	// Sega – CD / Saturn / shared CUE
	".msu": {"genesis_plus_gx_libretro"},
	".cue": {"genesis_plus_gx_libretro", "pcsx_rearmed_libretro", "mednafen_saturn_libretro"},

	// Sony – PS1
	".iso": {"pcsx_rearmed_libretro", "beetle_psx_libretro"},
	".bin": {"pcsx_rearmed_libretro", "beetle_psx_libretro"},
	".chd": {"pcsx_rearmed_libretro", "beetle_psx_libretro"},

	// Sony – PSP
	".cso": {"ppsspp_libretro"},

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

	// Others
	".pce": {"mednafen_pce_fast_libretro", "mednafen_pce_libretro"},
	".sgx": {"mednafen_pce_fast_libretro"},
	".ws":  {"mednafen_wswan_libretro"},
	".wsc": {"mednafen_wswan_libretro"},
	".ngp": {"mednafen_ngp_libretro"},
	".ngc": {"mednafen_ngp_libretro"},

	// Pico-8
	".p8":  {"retro8_libretro"},
	".png": {constants.CoreRetro8},
}

// PlatformCoreMap maps common platform names or slugs to an ordered list
// of known-working libretro core base names.
var PlatformCoreMap = map[string][]string{
	"gb":           {"gambatte_libretro", "mgba_libretro", "sameboy_libretro"},
	"gbc":          {"gambatte_libretro", "mgba_libretro", "sameboy_libretro"},
	"gba":          {"mgba_libretro", "vba_next_libretro"},
	"nes":          {"nestopia_libretro", "fceumm_libretro", "mesen_libretro"},
	"snes":         {"snes9x_libretro", "bsnes_libretro"},
	"n64":          {"mupen64plus_next_libretro", "parallel_n64_libretro"},
	"nds":          {"melonds_libretro", "desmume_libretro"},
	"dsi":          {"melonds_libretro", "desmume_libretro"},
	"genesis":      {"genesis_plus_gx_libretro", "picodrive_libretro", "blastem_libretro"},
	"megadrive":    {"genesis_plus_gx_libretro", "picodrive_libretro", "blastem_libretro"},
	"mastersystem": {"genesis_plus_gx_libretro", "picodrive_libretro"},
	"gamegear":     {"genesis_plus_gx_libretro"},
	"psx":          {"pcsx_rearmed_libretro", "beetle_psx_libretro"},
	"ps1":          {"pcsx_rearmed_libretro", "beetle_psx_libretro"},
	"psp":          {"ppsspp_libretro"},
	"dreamcast":    {"flycast_libretro"},
	"pce":          {"mednafen_pce_fast_libretro", "mednafen_pce_libretro"},
	"gamecube":     {"dolphin_libretro"},
	"gcn":          {"dolphin_libretro"},
	"wii":          {"dolphin_libretro"},
	"3ds":          {constants.CoreCitra},
	"p8":           {"retro8_libretro"},
	"pico8":        {"retro8_libretro"},
	"wonderswan":   {"mednafen_wswan_libretro"},
	"wsc":          {"mednafen_wswan_libretro"},
	"ngp":          {"mednafen_ngp_libretro"},
	"ngpc":         {"mednafen_ngp_libretro"},
	"vb":           {"beetle_vb_libretro"},
	"virtualboy":   {"beetle_vb_libretro"},
	"lynx":         {"handy_libretro"},
	"pce_fast":     {"mednafen_pce_fast_libretro"},
	"supergrafx":   {"mednafen_pce_fast_libretro"},
}

// GetCoresForPlatform returns the ordered list of known-working libretro core
// base-names for the given platform slug or name.
func GetCoresForPlatform(platform string) []string {
	if platform == "" {
		return nil
	}
	// Try the direct mapping first.
	if cores, ok := PlatformCoreMap[strings.ToLower(platform)]; ok {
		return cores
	}
	// Fallback to fuzzy identification.
	slug := IdentifyPlatform(platform)
	if slug != "" {
		return PlatformCoreMap[slug]
	}
	return nil
}

// platformSearchPatterns defines fuzzy matching rules for identifying platforms from strings.
// Order matters: more specific patterns (e.g. "snes") should come before more general ones (e.g. "nes").
var platformSearchPatterns = []struct {
	slug     string
	patterns []string
	all      bool
}{
	{"gba", []string{"advance", "gba"}, false},
	{"3ds", []string{"3ds"}, false},
	{"gb", []string{"game boy", "gb"}, false},
	{"dsi", []string{"dsi"}, false},
	{"nds", []string{"ds", "nds"}, false},
	{"gamecube", []string{"gamecube", "gcn"}, false},
	{"wii", []string{"wii"}, false},
	{"genesis", []string{"genesis", "mega drive", "megadrive"}, false},
	{"wsc", []string{"wonderswan", "wsc"}, false},
	{"ngp", []string{"neo", "pocket"}, true},
	{"snes", []string{"snes"}, false},
	{"nes", []string{"nes"}, false},
	{"n64", []string{"n64"}, false},
	{"ps1", []string{"ps1", "psx"}, false},
	{"psp", []string{"psp"}, false},
	{"dreamcast", []string{"dreamcast"}, false},
	{"lynx", []string{"lynx"}, false},
	{"vb", []string{"virtual", "boy"}, true},
}

// IdentifyPlatform attempts to resolve a canonical platform slug from a string,
// such as a folder name or a tag (e.g., "Nintendo - Game Boy" -> "gb").
func IdentifyPlatform(input string) string {
	lower := strings.ToLower(input)
	if lower == "" || lower == "roms" {
		return ""
	}

	for _, entry := range platformSearchPatterns {
		matches := false
		if entry.all {
			matches = true
			for _, p := range entry.patterns {
				if !strings.Contains(lower, p) {
					matches = false
					break
				}
			}
		} else {
			for _, p := range entry.patterns {
				if strings.Contains(lower, p) {
					matches = true
					break
				}
			}
		}

		if matches {
			return entry.slug
		}
	}

	// Direct check as fallback
	if _, ok := PlatformCoreMap[lower]; ok {
		return lower
	}

	return ""
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

// getCoreExt returns the expected dynamic library extension for the current OS
func getCoreExt() string {
	switch runtime.GOOS {
	case constants.OSWindows:
		return ".dll"
	case constants.OSDarwin:
		return ".dylib"
	default: // linux, freebsd, etc
		return ".so"
	}
}

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

// Launch launches RetroArch for the given ROM path, given the selected executable.
// coreOverride, when non-empty, bypasses the CoreMap lookup and forces that specific core.
//
// temp file management, OS-specific path handling, cheevos config) that is intentionally kept together
// to preserve readability and avoid scattering related logic across many small functions.
//
//nolint:gocognit,gocyclo // See above
func Launch(ui UIProvider, exePath, romPath, cheevosUser, cheevosPass, coreOverride, platform string) error {
	// If exePath is a directory, try to find the actual executable inside it
	if info, err := os.Stat(exePath); err == nil && info.IsDir() {
		found := false
		target := filepath.Join(exePath, "retroarch.exe")
		if runtime.GOOS != constants.OSWindows && runtime.GOOS != constants.OSDarwin {
			target = filepath.Join(exePath, "retroarch")
		}

		if runtime.GOOS == constants.OSDarwin {
			if strings.HasSuffix(exePath, ".app") {
				found = true
			} else {
				appPath := filepath.Join(exePath, "RetroArch.app")
				if _, err := os.Stat(appPath); err == nil {
					exePath = appPath
					found = true
				} else {
					target = filepath.Join(exePath, "RetroArch")
					if _, err := os.Stat(target); err == nil {
						exePath = target
						found = true
					}
				}
			}
		} else {
			if _, err := os.Stat(target); err == nil {
				exePath = target
				found = true
			}
		}

		if !found {
			return fmt.Errorf("retroarch executable not found in directory: %s", exePath)
		}
	} else if err != nil {
		return fmt.Errorf("retroarch executable not found: %s", exePath)
	}

	baseDir := filepath.Dir(exePath)
	if runtime.GOOS == constants.OSDarwin {
		if strings.HasSuffix(exePath, ".app") {
			// If they selected the macOS .app bundle, use it as baseDir and find actual binary
			baseDir = exePath
			exePath = filepath.Join(exePath, "Contents", "MacOS", "RetroArch")
		} else if strings.Contains(exePath, ".app/Contents/MacOS") {
			// If they selected the binary inside the .app bundle
			baseDir = filepath.Dir(filepath.Dir(filepath.Dir(exePath)))
		}
	}
	coresDir := filepath.Join(baseDir, "cores")

	// Store original ROM base directory for saves/states early, before we potentially move romPath to a temp file
	romBaseDir := filepath.Dir(romPath)
	if strings.Contains(romPath, "#") {
		// Handle path if accidentally passed with #
		romBaseDir = filepath.Dir(strings.Split(romPath, "#")[0])
	}

	// Normalize the platform slug
	platform = IdentifyPlatform(platform)

	// Determine Core from extension
	ext := strings.ToLower(filepath.Ext(romPath))
	var tempRomPath string

	// If it's a zip file, we peek inside to find the first recognizable ROM extension
	if ext == ".zip" {
		r, err := zip.OpenReader(romPath)
		if err != nil {
			ui.LogErrorf("Launch: Failed to open .zip archive as PKZIP: %v. Passing original path to RetroArch.", err)
		} else {
			defer fileio.Close(r, nil, "Launch: Failed to close zip reader")

			foundExt := ""
			// If we have a platform, check if there's a preferred core/ext set for it
			platformCores := GetCoresForPlatform(platform)

			for _, f := range r.File {
				if f.FileInfo().IsDir() {
					continue
				}
				innerExt := strings.ToLower(filepath.Ext(f.Name))

				// Check if this extension is recognizable
				innerCores := GetCoresForExt(innerExt)
				if len(innerCores) == 0 {
					continue
				}

				// If we have platform-specific cores, prioritize finding one that matches
				match := true
				if len(platformCores) > 0 {
					match = false
					for _, pc := range platformCores {
						for _, ic := range innerCores {
							if pc == ic {
								match = true
								break
							}
						}
						if match {
							break
						}
					}
				}

				if match {
					foundExt = innerExt
					// Special case: Pico-8 .png carts inside ZIPs need manual extraction to a .p8 extension
					// to prevent RetroArch from defaulting to its internal image-viewer core.
					if innerExt == ".png" && innerCores[0] == constants.CoreRetro8 {
						tmpFile, err := os.CreateTemp("", "pico8_*.p8")
						if err != nil {
							return fmt.Errorf("failed to create temporary file for pico-8 extraction: %v", err)
						}
						rc, err := f.Open()
						if err != nil {
							fileio.Close(tmpFile, ui.LogErrorf, "Launch: Failed to close temporary file")
							fileio.Remove(tmpFile.Name(), ui.LogErrorf)
							return fmt.Errorf("failed to open zip member for extraction: %v", err)
						}
						_, err = io.Copy(tmpFile, rc)
						fileio.Close(rc, nil, "Launch: Failed to close zip member")
						fileio.Close(tmpFile, ui.LogErrorf, "Launch: Failed to close temporary file")
						if err != nil {
							fileio.Remove(tmpFile.Name(), ui.LogErrorf)
							return fmt.Errorf("failed to extract pico-8 cart from zip: %v", err)
						}
						romPath = tmpFile.Name()
						tempRomPath = romPath
						ui.LogInfof("Launch: Manually extracted Pico-8 .png cart from ZIP to %s", romPath)
					} else {
						// RetroArch requires the path to be formatted as: path/to/rom.zip#internal_rom.abc
						romPath = fmt.Sprintf("%s#%s", romPath, f.Name)
					}
					break
				}
			}
			if foundExt == "" {
				ui.LogErrorf("Launch: Could not find a recognizable ROM file inside the .zip archive. Passing original path to RetroArch.")
			} else {
				ext = foundExt
			}
		}
	}

	// Resolve the core: use an explicit override if provided, otherwise look up CoreMap or PlatformCoreMap.
	var coreBaseName string
	if coreOverride != "" {
		coreBaseName = filepath.Base(filepath.Clean(coreOverride))
		if coreBaseName == "." || coreBaseName == ".." {
			coreBaseName = ""
		}
	} else if platform != "" {
		// Try platform-based lookup first
		if pCores := GetCoresForPlatform(platform); len(pCores) > 0 {
			coreBaseName = pCores[0]
		}
	}

	// Fallback to extension-based lookup if platform didn't resolve a core
	if coreBaseName == "" {
		var ok bool
		coreBaseName, ok = CoreMap[ext]
		if !ok {
			return fmt.Errorf("no default core mapping found for extension: %s", ext)
		}
	}

	coreFile := coreBaseName + getCoreExt()

	// macOS core directory standard
	if runtime.GOOS == constants.OSDarwin {
		homeDir, _ := os.UserHomeDir()
		coresDir = filepath.Join(homeDir, "Library", "Application Support", "RetroArch", "cores")
	}
	corePath := filepath.Join(coresDir, coreFile)

	// Detect the required arch once — used for both downloading and arch-mismatch checking.
	arch := detectRetroArchArch(ui, exePath)

	coreExists := false
	if _, err := os.Stat(corePath); err == nil {
		coreExists = true
	}

	if coreExists && runtime.GOOS == constants.OSDarwin {
		// Verify the existing core's architecture matches what RetroArch needs.
		// This handles the case where a core was downloaded for a different arch
		// (e.g. x86_64 via Rosetta) but RetroArch is now running natively (arm64).
		if !coreArchMatches(corePath, arch) {
			ui.LogInfof("Launch: Core %s is wrong architecture for %s — deleting and re-downloading.", coreFile, arch)
			fileio.Remove(corePath, ui.LogErrorf)
			coreExists = false
		}
	}

	if !coreExists {
		ui.EventsEmit("play-status", fmt.Sprintf("Emulator core %s not found locally. Attempting to download...", coreFile))
		if err := DownloadCore(ui, coreFile, coresDir, arch); err != nil {
			return fmt.Errorf("emulator core not found at %s and auto-download failed: %w", corePath, err)
		}
	}

	// Workaround for Pico-8 .png carts being treated as images by RetroArch (physical files)
	if tempRomPath == "" && !strings.Contains(romPath, "#") && strings.ToLower(filepath.Ext(romPath)) == ".png" && coreBaseName == constants.CoreRetro8 {
		target := romPath + ".p8"
		// Remove existing if it somehow exists
		fileio.Remove(target, ui.LogErrorf)
		if err := os.Link(romPath, target); err == nil {
			ui.LogInfof("Launch: Created temporary hardlink %s for Pico-8 .png cart", target)
			romPath = target
			tempRomPath = target
		} else {
			ui.LogErrorf("Launch: Failed to create temporary hardlink: %v. Falling back to original path.", err)
		}
	}

	savesDir := filepath.Join(romBaseDir, constants.DirSaves)
	statesDir := filepath.Join(romBaseDir, constants.DirStates)
	ui.LogInfof("Launch: Saves dir: %s, States dir: %s", savesDir, statesDir)

	// Ensure directories exist
	fileio.MkdirAll(savesDir, 0o755, ui.LogErrorf)
	fileio.MkdirAll(statesDir, 0o755, ui.LogErrorf)

	// Prepare temporary config for RetroAchievements and Directories.
	// We use --appendconfig to pass these settings without modifying the user's main RetroArch config permanently.
	var appendConfigPath string
	tmpFile, err := os.CreateTemp("", "retroarch_config_*.cfg")
	if err == nil {
		appendConfigPath = tmpFile.Name()
		content := fmt.Sprintf("savefile_directory = %q\nsavestate_directory = %q\n", savesDir, statesDir)
		if cheevosUser != "" && cheevosPass != "" {
			content += fmt.Sprintf("cheevos_enable = \"true\"\ncheevos_username = %q\ncheevos_password = %q\n",
				cheevosUser, cheevosPass)
		}
		// Ensure RetroArch doesn't save these temporary paths back to the main config on exit
		content += "config_save_on_exit = \"false\"\n"

		if _, err := tmpFile.WriteString(content); err != nil {
			ui.LogErrorf("Launch: Failed to write temporary config: %v", err)
		}
		fileio.Close(tmpFile, ui.LogErrorf, "Launch: Failed to close temporary config file")
		ui.LogInfof("Launch: Created temporary config at: %s with content:\n%s", appendConfigPath, content)
	}

	fmt.Fprintln(os.Stderr, "--- PRE-LAUNCH CHECK ---")
	fmt.Fprintf(os.Stderr, "Exe: '%s'\nCore: '%s'\nROM: '%s'\nSaves: '%s'\nStates: '%s'\nAppend: '%s'\n",
		exePath, corePath, romPath, savesDir, statesDir, appendConfigPath)

	args := []string{"-L", corePath, "-f", "-v"}
	if appendConfigPath != "" {
		args = append(args, "--appendconfig", appendConfigPath)
	}
	args = append(args, romPath)

	cmd := exec.Command(exePath, args...)
	cmd.Dir = baseDir // run in the retroarch dir so it finds its config

	// Run in a goroutine so we don't block the Wails UI, but we can capture the output
	go func() {
		defer func() {
			if appendConfigPath != "" {
				fileio.Remove(appendConfigPath, ui.LogErrorf)
			}
			if tempRomPath != "" {
				fileio.Remove(tempRomPath, ui.LogErrorf)
			}
			ui.EventsEmit(constants.EventGameExited, nil)
			if runtime.GOOS == constants.OSDarwin {
				ui.WindowShow()
				ui.WindowUnminimise()
			}
		}()

		ui.EventsEmit(constants.EventGameStarted, nil)
		if runtime.GOOS == constants.OSDarwin {
			ui.WindowHide()
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("\n--- RETROARCH CRASHED ---\nError: %v\nOutput: %s\n", err, string(out))
		} else {
			fmt.Printf("\n--- RETROARCH EXITED ---\nOutput: %s\n", string(out))
		}
	}()

	// We return nil immediately since it's running detached
	return nil
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

	urlStr := fmt.Sprintf("https://buildbot.libretro.com/nightly/%s/%s/latest/%s.zip", osName, archName, coreFile)

	resp, err := http.Get(urlStr) //nolint:bodyclose // body is closed via fileio.Close wrapper below
	if err != nil {
		return fmt.Errorf("failed to download core: %w", err)
	}
	defer fileio.Close(resp.Body, nil, "DownloadCore: Failed to close response body")

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
			fileio.MkdirAll(fpath, os.ModePerm, nil)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
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

// ClearCheevosToken finds the RetroArch config file and clears the cheevos_token setting.
// This ensures that when credentials are changed, RetroArch will re-authenticate.
func ClearCheevosToken(exePath string) error {
	var configPaths []string

	// 1. Path based on exe directory or provided directory
	if exePath != "" {
		if info, err := os.Stat(exePath); err == nil {
			if info.IsDir() {
				configPaths = append(configPaths, filepath.Join(exePath, "retroarch.cfg"))
			} else {
				configPaths = append(configPaths, filepath.Join(filepath.Dir(exePath), "retroarch.cfg"))
			}
		}
	}

	// 2. Standard OS-specific locations
	switch runtime.GOOS {
	case constants.OSLinux:
		if home, err := os.UserHomeDir(); err == nil {
			configPaths = append(configPaths, filepath.Join(home, ".config", "retroarch", "retroarch.cfg"))
		}
	case constants.OSDarwin:
		if home, err := os.UserHomeDir(); err == nil {
			configPaths = append(configPaths, filepath.Join(home, "Library", "Application Support", "RetroArch", "config", "retroarch.cfg"))
		}
	}

	// Matches the line starting with cheevos_token = (case-insensitive, allowing leading whitespace)
	re := regexp.MustCompile(`(?mi)^\s*cheevos_token\s*=\s*.*`)

	// Try to find and clear the token in each potential config path
	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			content, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			newContent := re.ReplaceAllString(string(content), `cheevos_token = ""`)

			if string(content) != newContent {
				err = os.WriteFile(path, []byte(newContent), 0o644)
				if err != nil {
					return fmt.Errorf("failed to write updated retroarch.cfg at %s: %w", path, err)
				}
			}
		}
	}
	return nil
}

// isAppleSilicon returns true if the current host is running on Apple Silicon hardware,
// regardless of whether the current process is running via Rosetta.
func isAppleSilicon() bool {
	if runtime.GOOS != constants.OSDarwin {
		return false
	}
	// sysctl -n hw.optional.arm64 returns 1 on Apple Silicon
	out, err := exec.Command("sysctl", "-n", "hw.optional.arm64").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "1"
}

// detectRetroArchArch returns the Go-style architecture constant (e.g. "arm64", "amd64")
// that should be used when downloading cores, based on the RetroArch binary itself.
// On macOS this inspects the binary so Rosetta installs are handled correctly.
// On other platforms it falls back to runtime.GOARCH.
func detectRetroArchArch(ui UIProvider, exePath string) string {
	arch := runtime.GOARCH
	if runtime.GOOS != constants.OSDarwin {
		return arch
	}
	out, err := exec.Command("file", exePath).Output()
	if err != nil {
		return arch
	}
	sout := string(out)
	hasX86 := strings.Contains(sout, "x86_64")
	hasARM := strings.Contains(sout, "arm64")
	switch {
	case hasARM && hasX86:
		// Universal binary — prefer arm64 on Apple Silicon hardware.
		if isAppleSilicon() {
			arch = constants.ArchArm64
		} else {
			arch = constants.ArchAmd64
		}
	case hasARM:
		arch = constants.ArchArm64
	case hasX86:
		arch = constants.ArchAmd64
	}
	if ui != nil {
		ui.LogInfof("Launch: Detected RetroArch architecture: %s (ARM=%v, X86=%v)", arch, hasARM, hasX86)
	}
	return arch
}

// coreArchMatches returns true if the dylib at corePath is compiled for the given
// Go-style arch ("arm64" or "amd64"). Only meaningful on Darwin; always returns
// true on other platforms so we don't block non-macOS installs.
func coreArchMatches(corePath, arch string) bool {
	if runtime.GOOS != constants.OSDarwin {
		return true
	}
	out, err := exec.Command("file", corePath).Output()
	if err != nil {
		// Can't determine — assume it's fine to avoid a boot loop.
		return true
	}
	sout := string(out)
	switch arch {
	case constants.ArchArm64:
		return strings.Contains(sout, "arm64")
	case constants.ArchAmd64:
		return strings.Contains(sout, "x86_64")
	default:
		return true
	}
}
