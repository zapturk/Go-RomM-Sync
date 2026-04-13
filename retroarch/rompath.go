package retroarch

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go-romm-sync/constants"
	"go-romm-sync/utils/fileio"
)

// resolveRomPath inspects the ROM file and returns:
//   - ext: the effective file extension to use for core lookup
//   - romPath: the (possibly rewritten) path to pass to RetroArch
//   - tempRomPath: a temp file that must be cleaned up after launch (may be "")
//   - err: any fatal error
//
// For ZIP files it peeks inside to find the real ROM extension and rewrites
// romPath to the "archive.zip#inner_file.ext" format RetroArch expects.
// Pico-8 .png carts inside ZIPs are extracted to a real temp file.
func resolveRomPath(ui UIProvider, romPath, platform string) (ext, outRomPath, tempRomPath string, err error) {
	ext = strings.ToLower(filepath.Ext(romPath))
	outRomPath = romPath

	if ext != extZip {
		return ext, outRomPath, "", nil
	}

	r, openErr := zip.OpenReader(romPath)
	if openErr != nil {
		ui.LogErrorf("resolveRomPath: Failed to open .zip archive: %v. Passing original path to RetroArch.", openErr)
		return ext, outRomPath, "", nil
	}
	defer fileio.Close(r, nil, "resolveRomPath: Failed to close zip reader")

	platformCores := GetCoresForPlatform(platform)
	foundExt := ""

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		innerExt := strings.ToLower(filepath.Ext(f.Name))
		innerCores := GetCoresForExt(innerExt)
		if len(innerCores) == 0 {
			continue
		}

		if !coresOverlap(platformCores, innerCores) {
			continue
		}

		foundExt = innerExt

		// Special case: Pico-8 .png carts must be extracted to a .p8 temp file
		// to prevent RetroArch from routing them to the image-viewer core.
		if innerExt == ".png" && innerCores[0] == constants.CoreRetro8 {
			extracted, extractErr := extractZipMember(f, "pico8_*.p8")
			if extractErr != nil {
				return ext, romPath, "", extractErr
			}
			ui.LogInfof("resolveRomPath: Extracted Pico-8 .png cart from ZIP to %s", extracted)
			return innerExt, extracted, extracted, nil
		}

		// Standard case: use RetroArch's zip#inner_file notation
		outRomPath = fmt.Sprintf("%s#%s", romPath, f.Name)
		break
	}

	if foundExt == "" {
		ui.LogErrorf("resolveRomPath: No recognizable ROM found inside ZIP. Passing original path to RetroArch.")
	} else {
		ext = foundExt
	}

	return ext, outRomPath, "", nil
}

// coresOverlap returns true when platformCores is empty (no constraint) or when
// at least one core appears in both slices.
func coresOverlap(platformCores, innerCores []string) bool {
	if len(platformCores) == 0 {
		return true
	}
	for _, pc := range platformCores {
		for _, ic := range innerCores {
			if pc == ic {
				return true
			}
		}
	}
	return false
}

// extractZipMember extracts a single zip.File to a new OS temp file whose name
// matches the given pattern. The caller is responsible for removing the file.
func extractZipMember(f *zip.File, pattern string) (string, error) {
	tmpFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file for extraction: %w", err)
	}

	rc, err := f.Open()
	if err != nil {
		fileio.Close(tmpFile, nil, "extractZipMember: close tmp")
		fileio.Remove(tmpFile.Name(), nil)
		return "", fmt.Errorf("failed to open zip member for extraction: %w", err)
	}

	_, copyErr := io.Copy(tmpFile, rc)
	fileio.Close(rc, nil, "extractZipMember: close zip member")
	fileio.Close(tmpFile, nil, "extractZipMember: close tmp file")

	if copyErr != nil {
		fileio.Remove(tmpFile.Name(), nil)
		return "", fmt.Errorf("failed to extract zip member: %w", copyErr)
	}
	return tmpFile.Name(), nil
}

// httpGet is a thin wrapper around http.Get so launcher.go doesn't need to
// import net/http directly.
func httpGet(url string) (io.ReadCloser, error) {
	resp, err := http.Get(url) //nolint:bodyclose // caller closes
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return resp.Body, nil
}

// copyIO copies from src to dst.
func copyIO(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}
