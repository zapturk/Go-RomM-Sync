package retroarch

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

// Launch launches RetroArch for the given ROM path, given the selected executable.
// coreOverride, when non-empty, bypasses the CoreMap lookup and forces that specific core.
//
// temp file management, OS-specific path handling, cheevos config) that is intentionally kept together
// to preserve readability and avoid scattering related logic across many small functions.
//
//nolint:gocognit,gocyclo // See above
func Launch(ui UIProvider, exePath, romPath, cheevosUser, cheevosPass, coreOverride, platform, customBiosDir string) error {
	baseDir, resolvedExePath, err := resolveRetroArchPaths(exePath)
	if err != nil {
		return err
	}
	exePath = resolvedExePath

	coresDir := getCoresDir(baseDir)

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

	// coresDir is already set
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
			if strings.Contains(err.Error(), "status 404") {
				ui.EventsEmit(constants.EventPlayStatus, "Core Not Supported")
				ui.LogErrorf("Launch: Core %s not found on buildbot for %s/%s", coreFile, runtime.GOOS, arch)
				return fmt.Errorf("core not supported: %s", coreFile)
			}
			return fmt.Errorf("emulator core not found at %s and auto-download failed: %w", corePath, err)
		}
	}

	if coreBaseName == "pcsx2_libretro" {
		systemDir := GetSystemDir(baseDir)
		resourcesDir := filepath.Join(systemDir, "pcsx2", "resources")
		yamlPath := filepath.Join(resourcesDir, "GameIndex.yaml")

		if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
			fileio.MkdirAll(resourcesDir, 0o755, ui.LogErrorf)
			ui.EventsEmit(constants.EventPlayStatus, "Downloading PCSX2 GameIndex.yaml...")
			resp, err := http.Get("https://raw.githubusercontent.com/libretro/ps2/refs/heads/libretroization/bin/resources/GameIndex.yaml") //nolint:bodyclose // body is closed below
			if err == nil {
				if resp.StatusCode == http.StatusOK {
					out, createErr := os.Create(yamlPath)
					if createErr != nil {
						ui.LogErrorf("Launch: Failed to create GameIndex.yaml: %v", createErr)
					} else {
						if _, copyErr := io.Copy(out, resp.Body); copyErr != nil {
							ui.LogErrorf("Launch: Failed to write to GameIndex.yaml: %v", copyErr)
						}
						fileio.Close(out, ui.LogErrorf, "Launch: Failed to close GameIndex.yaml")
					}
				} else {
					ui.LogErrorf("Launch: Failed to download GameIndex.yaml, status code: %d", resp.StatusCode)
				}
				fileio.Close(resp.Body, nil, "Launch: Failed to close GameIndex response")
			} else {
				ui.LogErrorf("Launch: Failed to download GameIndex.yaml: %v", err)
			}
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
	// Default to RetroArch's internal system folder
	systemDir := GetSystemDir(baseDir)

	// If a custom firmware exists for this platform, use it
	if customBiosDir != "" && platform != "" {
		canonicalNames := GetBiosFilenamesForPlatform(platform)
		for _, name := range canonicalNames {
			subDir := ""
			if platform == "ps2" {
				subDir = filepath.Join("pcsx2", "bios")
			}
			if _, err := os.Stat(filepath.Join(customBiosDir, subDir, name)); err == nil {
				systemDir = customBiosDir
				ui.LogInfof("Launch: Found custom platform firmware for %s, using custom BIOS dir: %s", platform, systemDir)
				break
			}
		}
	}

	fileio.MkdirAll(systemDir, 0o755, ui.LogErrorf)

	// Prepare temporary config for RetroAchievements and Directories.
	// We use --appendconfig to pass these settings without modifying the user's main RetroArch config permanently.
	var appendConfigPath string
	tmpDir := filepath.Dir(coresDir)
	tmpFile, err := os.CreateTemp(tmpDir, "retroarch_config_*.cfg")
	if err == nil {
		appendConfigPath = tmpFile.Name()
		content := fmt.Sprintf("savefile_directory = %q\nsavestate_directory = %q\nsystem_directory = %q\n", savesDir, statesDir, systemDir)
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
		output, err := cmd.CombinedOutput()
		if err != nil {
			ui.LogErrorf("RetroArch failed with error: %v", err)
		}
		if len(output) > 0 {
			ui.LogInfof("RetroArch Output:\n%s", string(output))
		}
	}()

	// We return nil immediately since it's running detached
	return nil
}
