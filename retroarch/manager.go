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

var CoreMap = map[string]string{
	// Nintendo
	".nes": "nestopia_libretro",         // NES
	".fds": "nestopia_libretro",         // Famicom Disk System
	".sfc": "snes9x_libretro",           // SNES
	".smc": "snes9x_libretro",           // SNES
	".z64": "mupen64plus_next_libretro", // N64
	".n64": "mupen64plus_next_libretro", // N64
	".v64": "mupen64plus_next_libretro", // N64
	".gb":  "gambatte_libretro",         // GameBoy
	".gbc": "gambatte_libretro",         // GameBoy Color
	".gba": "mgba_libretro",             // GameBoy Advance
	".nds": "melonds_libretro",          // Nintendo DS
	".vb":  "beetle_vb_libretro",        // Virtual Boy

	// Sega
	".md":  "genesis_plus_gx_libretro", // MegaDrive / Genesis
	".smd": "genesis_plus_gx_libretro", // MegaDrive / Genesis
	".gen": "genesis_plus_gx_libretro", // MegaDrive / Genesis
	".sms": "genesis_plus_gx_libretro", // Master System
	".gg":  "genesis_plus_gx_libretro", // Game Gear
	".32x": "picodrive_libretro",       // 32X
	".msu": "genesis_plus_gx_libretro", // Sega CD
	".cue": "genesis_plus_gx_libretro", // Sega CD / Saturn / PS1 (Shared extension, mapped to GenPlus by default)

	// Sony
	".iso": "pcsx_rearmed_libretro", // PS1 / PSP (Shared, handling as PS1 by default)
	".bin": "pcsx_rearmed_libretro", // PS1
	".chd": "pcsx_rearmed_libretro", // PS1
	".cso": "ppsspp_libretro",       // PSP

	// Atari
	".a26": "stella_libretro",        // 2600
	".a52": "a5200_libretro",         // 5200
	".a78": "prosystem_libretro",     // 7800
	".lnx": "handy_libretro",         // Lynx
	".jag": "virtualjaguar_libretro", // Jaguar

	// Computers
	".d64": "vice_x64sc_libretro", // C64
	".prg": "vice_x64sc_libretro", // C64
	".t64": "vice_x64sc_libretro", // C64
	".adf": "puae_libretro",       // Amiga
	".uae": "puae_libretro",       // Amiga

	// Others
	".pce": "mednafen_pce_fast_libretro", // PC Engine
	".sgx": "mednafen_pce_fast_libretro", // PC Engine SuperGrafx
	".ws":  "mednafen_wswan_libretro",    // WonderSwan
	".wsc": "mednafen_wswan_libretro",    // WonderSwan Color
	".ngp": "mednafen_ngp_libretro",      // Neo Geo Pocket
	".ngc": "mednafen_ngp_libretro",      // Neo Geo Pocket Color

	// Nintendo GameCube & Wii
	".gcm":  "dolphin_libretro", // GameCube
	".gcz":  "dolphin_libretro", // GameCube
	".rvz":  "dolphin_libretro", // GameCube / Wii
	".wbfs": "dolphin_libretro", // Wii
	".wia":  "dolphin_libretro", // Wii

	// Pico-8
	".p8":  "retro8_libretro",    // Pico-8
	".png": constants.CoreRetro8, // Pico-8 (Cartridges)
}

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

// Launch launches RetroArch for the given ROM path, given the selected executable.
func Launch(ui UIProvider, exePath, romPath, cheevosUser, cheevosPass string) error {
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

	// Determine Core from extension
	ext := strings.ToLower(filepath.Ext(romPath))
	var tempRomPath string

	// If it's a zip file, we peek inside to find the first recognizable ROM extension
	if ext == ".zip" {
		r, err := zip.OpenReader(romPath)
		if err != nil {
			return fmt.Errorf("failed to open .zip rom archive: %v", err)
		}
		defer r.Close()

		foundExt := ""
		for _, f := range r.File {
			if f.FileInfo().IsDir() {
				continue
			}
			innerExt := strings.ToLower(filepath.Ext(f.Name))
			if coreName, ok := CoreMap[innerExt]; ok {
				foundExt = innerExt
				// Special case: Pico-8 .png carts inside ZIPs need manual extraction to a .p8 extension
				// to prevent RetroArch from defaulting to its internal image-viewer core.
				if innerExt == ".png" && coreName == constants.CoreRetro8 {
					tmpFile, err := os.CreateTemp("", "pico8_*.p8")
					if err != nil {
						return fmt.Errorf("failed to create temporary file for pico-8 extraction: %v", err)
					}
					rc, err := f.Open()
					if err != nil {
						tmpFile.Close()
						os.Remove(tmpFile.Name())
						return fmt.Errorf("failed to open zip member for extraction: %v", err)
					}
					_, err = io.Copy(tmpFile, rc)
					rc.Close()
					tmpFile.Close()
					if err != nil {
						os.Remove(tmpFile.Name())
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
			return fmt.Errorf("could not find a recognizable ROM file inside the .zip archive")
		}
		ext = foundExt
	}

	coreBaseName, ok := CoreMap[ext]
	if !ok {
		return fmt.Errorf("no default core mapping found for extension: %s", ext)
	}

	coreFile := coreBaseName + getCoreExt()

	// macOS core directory standard
	if runtime.GOOS == constants.OSDarwin {
		homeDir, _ := os.UserHomeDir()
		coresDir = filepath.Join(homeDir, "Library", "Application Support", "RetroArch", "cores")
	}
	corePath := filepath.Join(coresDir, coreFile)

	if _, err := os.Stat(corePath); err != nil {
		ui.EventsEmit("play-status", fmt.Sprintf("Emulator core %s not found locally. Attempting to download...", coreFile))

		// Detect architecture for macOS specifically to ensure we download the correct binary type.
		// On Apple Silicon (arm64), we might still be running an x86_64 build of RetroArch via Rosetta.
		arch := runtime.GOARCH
		if runtime.GOOS == constants.OSDarwin {
			// Try to detect the architecture of the RetroArch binary itself
			out, err := exec.Command("file", exePath).Output()
			if err == nil {
				sout := string(out)
				if strings.Contains(sout, "x86_64") {
					arch = constants.ArchAmd64
				} else if strings.Contains(sout, "arm64") {
					arch = constants.ArchArm64
				}
			}
		}

		err = DownloadCore(ui, coreFile, coresDir, arch)
		if err != nil {
			return fmt.Errorf("emulator core not found at %s and auto-download failed: %w", corePath, err)
		}
	}

	// Workaround for Pico-8 .png carts being treated as images by RetroArch (physical files)
	if tempRomPath == "" && !strings.Contains(romPath, "#") && strings.ToLower(filepath.Ext(romPath)) == ".png" && coreBaseName == constants.CoreRetro8 {
		target := romPath + ".p8"
		// Remove existing if it somehow exists
		os.Remove(target)
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
	os.MkdirAll(savesDir, 0o755)
	os.MkdirAll(statesDir, 0o755)

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
		tmpFile.Close()
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
				os.Remove(appendConfigPath)
			}
			if tempRomPath != "" {
				os.Remove(tempRomPath)
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

	resp, err := http.Get(urlStr)
	if err != nil {
		return fmt.Errorf("failed to download core: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("core download failed with status %d from %s", resp.StatusCode, urlStr)
	}

	os.MkdirAll(coresDir, 0o755)
	zipPath := filepath.Join(coresDir, coreFile+".zip")
	out, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("failed to create core zip: %w", err)
	}
	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		return fmt.Errorf("failed to save core zip: %w", err)
	}
	defer os.Remove(zipPath)

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
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
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
			outFile.Close()
			return err
		}
		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
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
