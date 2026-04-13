package retroarch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"go-romm-sync/constants"
)

var overrideCoresDir string

func getCoresDir(baseDir string) string {
	if overrideCoresDir != "" {
		return overrideCoresDir
	}

	if homeDir, err := os.UserHomeDir(); err == nil {
		switch runtime.GOOS {
		case constants.OSDarwin:
			return filepath.Join(homeDir, "Library", "Application Support", "RetroArch", "cores")
		case constants.OSLinux:
			// Attempt to identify package manager based on baseDir
			if strings.HasPrefix(baseDir, "/snap/") {
				return filepath.Join(homeDir, "snap", "retroarch", "current", ".config", "retroarch", "cores")
			}
			if strings.Contains(baseDir, "flatpak") {
				return filepath.Join(homeDir, ".var", "app", "org.libretro.RetroArch", "config", "retroarch", "cores")
			}
			// Native packages, AppImages, self-compiled binaries, etc.
			return filepath.Join(homeDir, ".config", "retroarch", "cores")
		}
	}

	// Fallback for Windows or if HomeDir fails
	return filepath.Join(baseDir, "cores")
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
	if home, err := os.UserHomeDir(); err == nil {
		switch runtime.GOOS {
		case constants.OSLinux:
			configPaths = append(configPaths, filepath.Join(home, ".config", "retroarch", "retroarch.cfg"))
		case constants.OSDarwin:
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

// resolveRetroArchPaths attempts to find the actual executable and its base
// directory given a user-provided file or directory path.
func resolveRetroArchPaths(exePath string) (baseDir, binaryPath string, err error) {
	// If exePath is a directory, try to find the actual executable inside it
	if info, statErr := os.Stat(exePath); statErr == nil && info.IsDir() {
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
				if _, statErr := os.Stat(appPath); statErr == nil {
					exePath = appPath
					found = true
				} else {
					target = filepath.Join(exePath, "RetroArch")
					if _, statErr := os.Stat(target); statErr == nil {
						exePath = target
						found = true
					}
				}
			}
		} else {
			if _, statErr := os.Stat(target); statErr == nil {
				exePath = target
				found = true
			}
		}

		if !found {
			return "", "", fmt.Errorf("retroarch executable not found in directory: %s", exePath)
		}
	} else if statErr != nil {
		return "", "", fmt.Errorf("retroarch executable not found: %s", exePath)
	}

	binaryPath = exePath
	baseDir = filepath.Dir(exePath)
	if runtime.GOOS == constants.OSDarwin {
		if strings.HasSuffix(exePath, ".app") {
			// If they selected the macOS .app bundle, use it as baseDir and find actual binary
			baseDir = exePath
			binaryPath = filepath.Join(exePath, "Contents", "MacOS", "RetroArch")
		} else if strings.Contains(exePath, ".app/Contents/MacOS") {
			// If they selected the binary inside the .app bundle
			baseDir = filepath.Dir(filepath.Dir(filepath.Dir(exePath)))
		}
	}
	return baseDir, binaryPath, nil
}

func GetSystemDir(baseDir string) string {
	if homeDir, err := os.UserHomeDir(); err == nil {
		switch runtime.GOOS {
		case constants.OSDarwin:
			return filepath.Join(homeDir, "Library", "Application Support", "RetroArch", "system")
		case constants.OSLinux:
			if strings.HasPrefix(baseDir, "/snap/") {
				return filepath.Join(homeDir, "snap", "retroarch", "current", ".config", "retroarch", "system")
			}
			if strings.Contains(baseDir, "flatpak") {
				return filepath.Join(homeDir, ".var", "app", "org.libretro.RetroArch", "config", "retroarch", "system")
			}
			return filepath.Join(homeDir, ".config", "retroarch", "system")
		}
	}
	return filepath.Join(baseDir, "system")
}
