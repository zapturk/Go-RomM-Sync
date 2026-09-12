package retroarch

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/bodgit/sevenzip"

	"go-romm-sync/constants"
)

const defaultStableVersion = "1.22.2"

var buildbotStableURL = constants.URLBuildbotStable

// installerProgressWriter tracks download bytes and emits percentage events.
type installerProgressWriter struct {
	total       int64
	downloaded  int64
	ui          UIProvider
	lastPercent int
}

func (pw *installerProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.downloaded += int64(n)
	if pw.total > 0 {
		percent := int(float64(pw.downloaded) / float64(pw.total) * 100)
		if percent > 100 {
			percent = 100
		}
		if percent > pw.lastPercent {
			pw.lastPercent = percent
			if pw.ui != nil {
				pw.ui.EventsEmit("retroarch-install-progress", percent)
				if percent%10 == 0 || percent == 100 {
					pw.ui.EventsEmit(constants.EventPlayStatus, fmt.Sprintf("Downloading RetroArch (%d%%)...", percent))
				}
			}
		}
	}
	return n, nil
}

// getRetroArchDownloadURL constructs the official Libretro buildbot download URL for the current OS/architecture.
func getRetroArchDownloadURL(version, goos, goarch string) (string, error) {
	baseURL := fmt.Sprintf("%s/%s", buildbotStableURL, version)
	switch goos {
	case constants.OSDarwin:
		return fmt.Sprintf("%s/apple/osx/universal/RetroArch_Metal.dmg", baseURL), nil
	case constants.OSWindows:
		if goarch == constants.Arch386 {
			return fmt.Sprintf("%s/windows/x86/RetroArch.7z", baseURL), nil
		}
		return fmt.Sprintf("%s/windows/x86_64/RetroArch.7z", baseURL), nil
	case constants.OSLinux:
		if goarch == constants.Arch386 {
			return fmt.Sprintf("%s/linux/x86/RetroArch.7z", baseURL), nil
		}
		return fmt.Sprintf("%s/linux/x86_64/RetroArch.7z", baseURL), nil
	default:
		return "", fmt.Errorf("unsupported operating system for RetroArch download: %s", goos)
	}
}

// getDefaultInstallDir returns the base installation directory for the current operating system.
func getDefaultInstallDir(goos string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	switch goos {
	case constants.OSDarwin:
		// Check if /Applications is writable
		appDir := "/Applications"
		testFile := filepath.Join(appDir, ".test_write_romm")
		if err := os.WriteFile(testFile, []byte(""), 0o644); err == nil {
			_ = os.Remove(testFile)
			return appDir, nil
		}
		// Fallback to ~/Applications
		userAppDir := filepath.Join(homeDir, "Applications")
		_ = os.MkdirAll(userAppDir, 0o755)
		return userAppDir, nil

	case constants.OSWindows:
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			localAppData = filepath.Join(homeDir, "AppData", "Local")
		}
		return filepath.Join(localAppData, "RetroArch"), nil

	case constants.OSLinux:
		return filepath.Join(homeDir, ".local", "share", "RetroArch"), nil

	default:
		return "", fmt.Errorf("unsupported OS: %s", goos)
	}
}

// DownloadAndInstall downloads, installs, and returns the executable path for RetroArch.
func DownloadAndInstall(ui UIProvider) (string, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	version := defaultStableVersion
	downloadURL, err := getRetroArchDownloadURL(version, goos, goarch)
	if err != nil {
		return "", err
	}

	if ui != nil {
		ui.LogInfof("Downloading RetroArch version %s from %s", version, downloadURL)
		ui.EventsEmit(constants.EventPlayStatus, fmt.Sprintf("Connecting to download RetroArch %s...", version))
	}

	// Create a temporary file for the download
	tmpPattern := "retroarch_dl_*.dmg"
	if goos != constants.OSDarwin {
		tmpPattern = "retroarch_dl_*.7z"
	}

	tmpFile, err := os.CreateTemp("", tmpPattern)
	if err != nil {
		return "", fmt.Errorf("failed to create temp download file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}()

	if err := downloadArchive(ui, downloadURL, tmpFile); err != nil {
		return "", err
	}
	_ = tmpFile.Close()

	targetBaseDir, err := getDefaultInstallDir(goos)
	if err != nil {
		return "", err
	}

	var installedPath string
	switch goos {
	case constants.OSDarwin:
		installedPath, err = installDarwin(ui, tmpPath, targetBaseDir)
	case constants.OSWindows:
		installedPath, err = installWindows(ui, tmpPath, targetBaseDir)
	case constants.OSLinux:
		installedPath, err = installLinux(ui, tmpPath, targetBaseDir)
	default:
		return "", fmt.Errorf("unsupported operating system: %s", goos)
	}

	if err != nil {
		return "", err
	}

	if ui != nil {
		ui.EventsEmit(constants.EventPlayStatus, "RetroArch installed successfully!")
		ui.EventsEmit("retroarch-install-progress", 100)
		ui.LogInfof("RetroArch successfully installed at: %s", installedPath)
	}

	return installedPath, nil
}

// downloadArchive downloads a URL into destFile while reporting progress.
func downloadArchive(ui UIProvider, urlStr string, destFile *os.File) error {
	resp, err := httpDownloadClient.Get(urlStr)
	if err != nil {
		return fmt.Errorf("failed to download RetroArch: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with HTTP status %d from %s", resp.StatusCode, urlStr)
	}

	pw := &installerProgressWriter{
		total: resp.ContentLength,
		ui:    ui,
	}

	_, copyErr := io.Copy(io.MultiWriter(destFile, pw), resp.Body)
	if copyErr != nil {
		return fmt.Errorf("failed to save download: %w", copyErr)
	}

	return nil
}

// installDarwin mounts the DMG, copies RetroArch.app to destination, detaches, and clears quarantine.
func installDarwin(ui UIProvider, dmgPath, targetBaseDir string) (string, error) {
	if ui != nil {
		ui.EventsEmit(constants.EventPlayStatus, "Mounting RetroArch disk image...")
	}

	tmpMount, err := os.MkdirTemp("", "retroarch_mount_*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary mount directory: %w", err)
	}
	defer func() {
		_ = exec.Command("hdiutil", "detach", tmpMount, "-force", "-quiet").Run()
		_ = os.RemoveAll(tmpMount)
	}()

	// Mount DMG
	mountCmd := exec.Command("hdiutil", "attach", dmgPath, "-mountpoint", tmpMount, "-nobrowse", "-quiet")
	if out, err := mountCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to mount DMG: %v (output: %s)", err, string(out))
	}

	// Locate RetroArch.app inside mount point
	srcApp := filepath.Join(tmpMount, "RetroArch.app")
	if _, err := os.Stat(srcApp); err != nil {
		return "", fmt.Errorf("RetroArch.app not found inside DMG: %w", err)
	}

	if err := os.MkdirAll(targetBaseDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create destination directory %s: %w", targetBaseDir, err)
	}

	targetApp := filepath.Join(targetBaseDir, "RetroArch.app")
	if _, err := os.Stat(targetApp); err == nil {
		if ui != nil {
			ui.LogInfof("Removing existing RetroArch at %s", targetApp)
		}
		_ = os.RemoveAll(targetApp)
	}

	if ui != nil {
		ui.EventsEmit(constants.EventPlayStatus, "Copying RetroArch to Applications...")
	}

	// Use /usr/bin/ditto to preserve signatures, architectures, and attributes
	dittoCmd := exec.Command("/usr/bin/ditto", srcApp, targetApp)
	if out, err := dittoCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to copy RetroArch.app: %v (%s)", err, string(out))
	}

	// Strip Gatekeeper quarantine attribute
	_ = exec.Command("/usr/bin/xattr", "-dr", "com.apple.quarantine", targetApp).Run()

	return targetApp, nil
}

// installWindows extracts the 7z archive to targetDir and finds retroarch.exe.
func installWindows(ui UIProvider, archivePath, targetDir string) (string, error) {
	if ui != nil {
		ui.EventsEmit(constants.EventPlayStatus, "Extracting RetroArch...")
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create target directory %s: %w", targetDir, err)
	}

	if err := extract7zArchive(ui, archivePath, targetDir); err != nil {
		return "", fmt.Errorf("failed to extract RetroArch 7z: %w", err)
	}

	// Find retroarch.exe
	exePath := filepath.Join(targetDir, "retroarch.exe")
	if _, err := os.Stat(exePath); err != nil {
		return "", fmt.Errorf("retroarch.exe not found after extraction in %s: %w", targetDir, err)
	}

	return exePath, nil
}

// installLinux extracts the 7z archive to targetDir, marks the AppImage executable, and returns its path.
func installLinux(ui UIProvider, archivePath, targetDir string) (string, error) {
	if ui != nil {
		ui.EventsEmit(constants.EventPlayStatus, "Extracting RetroArch...")
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create target directory %s: %w", targetDir, err)
	}

	if err := extract7zArchive(ui, archivePath, targetDir); err != nil {
		return "", fmt.Errorf("failed to extract RetroArch 7z: %w", err)
	}

	// Search for AppImage or executable
	for _, candidate := range []string{"RetroArch.AppImage", "retroarch.AppImage", "retroarch"} {
		p := filepath.Join(targetDir, candidate)
		if _, err := os.Stat(p); err == nil {
			_ = os.Chmod(p, 0o755)
			return p, nil
		}
	}

	return "", fmt.Errorf("no RetroArch executable or AppImage found after extraction in %s", targetDir)
}

// detect7zCommonPrefix returns a common root directory prefix (e.g. "RetroArch-Win64/") if all files share it.
func detect7zCommonPrefix(files []*sevenzip.File) string {
	if len(files) == 0 {
		return ""
	}
	firstSlash := strings.Index(files[0].Name, "/")
	if firstSlash == -1 {
		return ""
	}
	candidate := files[0].Name[:firstSlash+1]
	for _, f := range files {
		if !strings.HasPrefix(f.Name, candidate) {
			return ""
		}
	}
	return candidate
}

func report7zProgress(ui UIProvider, current, total int, lastPercent *int) {
	if ui == nil || total == 0 {
		return
	}
	pct := int(float64(current) / float64(total) * 100)
	if pct > *lastPercent && pct%5 == 0 {
		*lastPercent = pct
		ui.EventsEmit("retroarch-install-progress", pct)
		ui.EventsEmit(constants.EventPlayStatus, fmt.Sprintf("Extracting RetroArch (%d%%)...", pct))
	}
}

func isExecutableEntry(name string) bool {
	lowerName := strings.ToLower(name)
	return strings.HasSuffix(lowerName, ".exe") ||
		strings.HasSuffix(lowerName, ".appimage") ||
		strings.HasSuffix(lowerName, "retroarch") ||
		strings.Contains(lowerName, "bin/")
}

func determine7zFileMode(f *sevenzip.File, relName string) os.FileMode {
	mode := f.Mode()
	if mode.Perm() == 0 {
		mode = 0o644
	}
	if isExecutableEntry(relName) {
		mode = 0o755
	}
	return mode
}

func extract7zFile(f *sevenzip.File, destPath string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", destPath, err)
	}

	outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("failed to open destination file %s: %w", destPath, err)
	}
	defer outFile.Close() //nolint:errcheck

	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("failed to open archive entry %s: %w", f.Name, err)
	}
	defer rc.Close() //nolint:errcheck

	if _, err := io.Copy(outFile, rc); err != nil {
		return fmt.Errorf("failed to extract file %s: %w", destPath, err)
	}

	return nil
}

// extract7zArchive extracts a .7z archive into destDir with progress reporting and common prefix stripping.
func extract7zArchive(ui UIProvider, archivePath, destDir string) error {
	r, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open 7z archive: %w", err)
	}
	defer r.Close() //nolint:errcheck

	total := len(r.File)
	lastPercent := -1
	commonPrefix := detect7zCommonPrefix(r.File)
	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)

	for i, f := range r.File {
		report7zProgress(ui, i, total, &lastPercent)

		relName := f.Name
		if commonPrefix != "" {
			relName = strings.TrimPrefix(relName, commonPrefix)
		}
		if relName == "" {
			continue
		}

		fpath := filepath.Join(destDir, relName)
		// Path traversal check
		if !strings.HasPrefix(filepath.Clean(fpath), cleanDest) {
			continue
		}

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(fpath, 0o755)
			continue
		}

		mode := determine7zFileMode(f, relName)
		if err := extract7zFile(f, fpath, mode); err != nil {
			return err
		}
	}

	return nil
}
