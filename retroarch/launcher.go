package retroarch

import (
	"fmt"
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

// Launch launches RetroArch for the given ROM path and selected executable.
// coreOverride, when non-empty, bypasses the CoreMap lookup and forces that specific core.
func Launch(ui UIProvider, exePath, romPath, cheevosUser, cheevosPass, coreOverride, platform, customBiosDir string) error {
	baseDir, resolvedExePath, err := resolveRetroArchPaths(exePath)
	if err != nil {
		return err
	}
	exePath = resolvedExePath
	coresDir := getCoresDir(baseDir)

	// Store original ROM base directory for saves/states before we potentially
	// rewrite romPath to a temp file or a zip-internal path.
	romBaseDir := filepath.Dir(romPath)
	if strings.Contains(romPath, "#") {
		romBaseDir = filepath.Dir(strings.Split(romPath, "#")[0])
	}

	platform = IdentifyPlatform(platform)

	// Resolve the actual ROM path (handles ZIP inspection, Pico-8 extraction, etc.)
	ext, romPath, tempRomPath, err := resolveRomPath(ui, romPath, platform)
	if err != nil {
		return err
	}

	// Resolve the core to use.
	coreBaseName, err := resolveCore(coreOverride, platform, ext)
	if err != nil {
		return err
	}

	coreFile := coreBaseName + getCoreExt()
	corePath := filepath.Join(coresDir, coreFile)
	arch := detectRetroArchArch(ui, exePath)

	if err := ensureCore(ui, corePath, coreFile, coresDir, arch); err != nil {
		return err
	}

	if err := ensurePCSX2Resources(ui, coreBaseName, baseDir); err != nil {
		ui.LogErrorf("Launch: PCSX2 resource setup failed: %v", err)
	}

	appendConfigPath := prepareLaunchEnv(ui, baseDir, romBaseDir, platform, customBiosDir, cheevosUser, cheevosPass)

	runRetroArch(ui, exePath, baseDir, corePath, romPath, appendConfigPath, tempRomPath)

	return nil
}

// runRetroArch executes the RetroArch process in a separate goroutine and handles
// the lifecycle events (started, exited, cleanup).
func runRetroArch(ui UIProvider, exePath, baseDir, corePath, romPath, appendConfigPath, tempRomPath string) {
	args := []string{"-L", corePath, "-f", "-v"}
	if appendConfigPath != "" {
		args = append(args, "--appendconfig", appendConfigPath)
	}
	args = append(args, romPath)

	cmd := exec.Command(exePath, args...)
	cmd.Dir = baseDir

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
}

// resolveCore picks the libretro core base name to use, in priority order:
// explicit override → platform lookup → extension lookup.
func resolveCore(coreOverride, platform, ext string) (string, error) {
	if coreOverride != "" {
		base := filepath.Base(filepath.Clean(coreOverride))
		if base != "." && base != ".." {
			return base, nil
		}
	}

	if platform != "" {
		if pCores := GetCoresForPlatform(platform); len(pCores) > 0 {
			return pCores[0], nil
		}
	}

	core, ok := CoreMap[ext]
	if !ok {
		return "", fmt.Errorf("no default core mapping found for extension: %s", ext)
	}
	return core, nil
}

// ensureCore verifies the core exists locally (and has the right arch on macOS),
// downloading it from the buildbot if necessary.
func ensureCore(ui UIProvider, corePath, coreFile, coresDir, arch string) error {
	coreExists := false
	if _, err := os.Stat(corePath); err == nil {
		coreExists = true
	}

	if coreExists && runtime.GOOS == constants.OSDarwin {
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
	return nil
}

// ensurePCSX2Resources downloads the PCSX2 GameIndex.yaml if it is missing.
func ensurePCSX2Resources(ui UIProvider, coreBaseName, baseDir string) error {
	if coreBaseName != "pcsx2_libretro" {
		return nil
	}
	systemDir := GetSystemDir(baseDir)
	yamlPath := filepath.Join(systemDir, "pcsx2", "resources", "GameIndex.yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		return nil
	}

	fileio.MkdirAll(filepath.Dir(yamlPath), 0o755, ui.LogErrorf)
	ui.EventsEmit(constants.EventPlayStatus, "Downloading PCSX2 GameIndex.yaml...")

	body, err := httpGet(constants.URLPCSX2GameIndex)
	if err != nil {
		return fmt.Errorf("failed to fetch GameIndex.yaml: %w", err)
	}
	defer fileio.Close(body, ui.LogErrorf, "ensurePCSX2Resources: close response")

	out, err := os.Create(yamlPath)
	if err != nil {
		return fmt.Errorf("failed to create GameIndex.yaml: %w", err)
	}
	defer fileio.Close(out, ui.LogErrorf, "ensurePCSX2Resources: close file")

	if _, err := copyIO(out, body); err != nil {
		return fmt.Errorf("failed to write GameIndex.yaml: %w", err)
	}
	return nil
}

// prepareLaunchEnv sets up the directories and config file needed for a RetroArch launch.
func prepareLaunchEnv(ui UIProvider, baseDir, romBaseDir, platform, customBiosDir, cheevosUser, cheevosPass string) string {
	savesDir := filepath.Join(romBaseDir, constants.DirSaves)
	statesDir := filepath.Join(romBaseDir, constants.DirStates)
	ui.LogInfof("Launch: Saves dir: %s, States dir: %s", savesDir, statesDir)
	fileio.MkdirAll(savesDir, 0o755, ui.LogErrorf)

	systemDir := resolveSystemDir(ui, baseDir, platform, customBiosDir)
	fileio.MkdirAll(systemDir, 0o755, ui.LogErrorf)

	return writeTempConfig(ui, savesDir, statesDir, systemDir, cheevosUser, cheevosPass)
}

// resolveSystemDir returns the RetroArch system directory, preferring a custom
// BIOS dir when the platform's firmware is present there.
func resolveSystemDir(ui UIProvider, baseDir, platform, customBiosDir string) string {
	systemDir := GetSystemDir(baseDir)
	if customBiosDir == "" || platform == "" {
		return systemDir
	}
	for _, name := range GetBiosFilenamesForPlatform(platform) {
		subDir := ""
		if platform == "ps2" {
			subDir = filepath.Join("pcsx2", "bios")
		}
		if _, err := os.Stat(filepath.Join(customBiosDir, subDir, name)); err == nil {
			ui.LogInfof("Launch: Found custom platform firmware for %s, using custom BIOS dir: %s", platform, customBiosDir)
			return customBiosDir
		}
	}
	return systemDir
}

// writeTempConfig writes a temporary RetroArch --appendconfig file and returns
// its path. Returns "" if the file could not be created (non-fatal).
func writeTempConfig(ui UIProvider, savesDir, statesDir, systemDir, cheevosUser, cheevosPass string) string {
	tmpFile, err := os.CreateTemp("", "retroarch_config_*.cfg")
	if err != nil {
		ui.LogErrorf("Launch: Failed to create temporary config: %v", err)
		return ""
	}

	content := fmt.Sprintf(
		"savefile_directory = %q\nsavestate_directory = %q\nsystem_directory = %q\n",
		savesDir, statesDir, systemDir,
	)
	if cheevosUser != "" && cheevosPass != "" {
		content += fmt.Sprintf(
			"cheevos_enable = \"true\"\ncheevos_username = %q\ncheevos_password = %q\n",
			cheevosUser, cheevosPass,
		)
	}
	content += "config_save_on_exit = \"false\"\n"

	if _, err := tmpFile.WriteString(content); err != nil {
		ui.LogErrorf("Launch: Failed to write temporary config: %v", err)
	}
	fileio.Close(tmpFile, ui.LogErrorf, "Launch: Failed to close temporary config file")
	ui.LogInfof("Launch: Created temporary config at: %s with content:\n%s", tmpFile.Name(), content)
	return tmpFile.Name()
}
