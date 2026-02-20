package retroarch

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var coreMap = map[string]string{
	// Nintendo
	".nes": "nestopia_libretro",         // NES
	".sfc": "snes9x_libretro",           // SNES
	".smc": "snes9x_libretro",           // SNES
	".z64": "mupen64plus_next_libretro", // N64
	".n64": "mupen64plus_next_libretro", // N64
	".v64": "mupen64plus_next_libretro", // N64
	".gb":  "gambatte_libretro",         // GameBoy
	".gbc": "gambatte_libretro",         // GameBoy Color
	".gba": "mgba_libretro",             // GameBoy Advance

	// Sega
	".md":  "genesis_plus_gx_libretro", // MegaDrive / Genesis
	".smd": "genesis_plus_gx_libretro", // MegaDrive / Genesis
	".gen": "genesis_plus_gx_libretro", // MegaDrive / Genesis
	".sms": "genesis_plus_gx_libretro", // Master System
	".gg":  "genesis_plus_gx_libretro", // Game Gear

	// Sony
	".iso": "pcsx_rearmed_libretro", // PS1 (also could be other CD systems, simplified for now)
	".bin": "pcsx_rearmed_libretro", // PS1
	".cue": "pcsx_rearmed_libretro", // PS1
}

// getCoreExt returns the expected dynamic library extension for the current OS
func getCoreExt() string {
	switch runtime.GOOS {
	case "windows":
		return ".dll"
	case "darwin":
		return ".dylib"
	default: // linux, freebsd, etc
		return ".so"
	}
}

// Launch launches RetroArch for the given ROM path, given the selected executable.
func Launch(ctx context.Context, exePath, romPath string) error {
	baseDir := filepath.Dir(exePath)
	if runtime.GOOS == "darwin" {
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

	// Determine Core from extension
	ext := strings.ToLower(filepath.Ext(romPath))

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
			if _, ok := coreMap[innerExt]; ok {
				foundExt = innerExt
				// RetroArch requires the path to be formatted as: path/to/rom.zip#internal_rom.abc
				romPath = fmt.Sprintf("%s#%s", romPath, f.Name)
				break
			}
		}

		if foundExt == "" {
			return fmt.Errorf("could not find a recognizable ROM file inside the .zip archive")
		}
		ext = foundExt
	}

	coreBaseName, ok := coreMap[ext]
	if !ok {
		return fmt.Errorf("no default core mapping found for extension: %s", ext)
	}

	coreFile := coreBaseName + getCoreExt()

	// macOS core directory standard
	if runtime.GOOS == "darwin" {
		homeDir, _ := os.UserHomeDir()
		coresDir = filepath.Join(homeDir, "Library", "Application Support", "RetroArch", "cores")
	}
	corePath := filepath.Join(coresDir, coreFile)

	if _, err := os.Stat(corePath); err != nil {
		wailsRuntime.EventsEmit(ctx, "play-status", fmt.Sprintf("Emulator core %s not found locally. Attempting to download...", coreFile))

		// Detect architecture for macOS specifically, otherwise use runtime.GOARCH
		arch := runtime.GOARCH
		if runtime.GOOS == "darwin" {
			// Try to detect the architecture of the binary
			out, err := exec.Command("file", exePath).Output()
			if err == nil {
				sout := string(out)
				if strings.Contains(sout, "x86_64") {
					arch = "amd64"
				} else if strings.Contains(sout, "arm64") {
					arch = "arm64"
				}
			}
		}

		err = DownloadCore(ctx, coreFile, coresDir, arch)
		if err != nil {
			return fmt.Errorf("emulator core not found at %s and auto-download failed: %w", corePath, err)
		}
	}

	// Launch Retroarch
	fmt.Printf("--- PRE-LAUNCH CHECK ---\nExe: '%s'\nCore: '%s'\nROM: '%s'\n", exePath, corePath, romPath)

	cmd := exec.Command(exePath, "-L", corePath, "-f", "-v", romPath)
	cmd.Dir = baseDir // run in the retroarch dir so it finds its config

	// Run in a goroutine so we don't block the Wails UI, but we can capture the output
	go func() {
		// Hide the Go-RomM-Sync window and disable its input while playing
		wailsRuntime.WindowHide(ctx)
		defer func() {
			wailsRuntime.WindowShow(ctx)
			// Bring to front on Windows/Linux (Wails APIs are sometimes finicky)
			// Unminimise, show, and briefly set AlwaysOnTop then toggle off to force Z-order
			wailsRuntime.WindowUnminimise(ctx)
			wailsRuntime.WindowSetAlwaysOnTop(ctx, true)
			wailsRuntime.WindowSetAlwaysOnTop(ctx, false)
		}()

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
func DownloadCore(ctx context.Context, coreFile, coresDir, arch string) error {
	wailsRuntime.EventsEmit(ctx, "play-status", fmt.Sprintf("Downloading missing core: %s...", coreFile))

	var osName, archName string
	switch runtime.GOOS {
	case "windows":
		osName = "windows"
	case "darwin":
		osName = "apple/osx"
	case "linux":
		osName = "linux"
	default:
		return fmt.Errorf("unsupported OS for core downloads: %s", runtime.GOOS)
	}

	switch arch {
	case "amd64":
		archName = "x86_64"
	case "arm64":
		if runtime.GOOS == "darwin" {
			archName = "arm64"
		} else {
			archName = "aarch64"
		}
	case "386":
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

	os.MkdirAll(coresDir, 0755)
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

	wailsRuntime.EventsEmit(ctx, "play-status", "Core downloaded successfully!")
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
		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
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
