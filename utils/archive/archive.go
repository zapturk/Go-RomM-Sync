package archive

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bodgit/sevenzip"
	"github.com/nwaples/rardecode/v2"
)

// ExtractCueBin checks if an archive contains .cue and .bin files and extracts them if it does.
// Returns true if files were extracted, false if not an archive or no .cue/.bin pair found.
func ExtractCueBin(src string, destDir string) (bool, error) {
	format := sniffFormat(src)
	if format == "" {
		// Fallback to extension if sniffing fails (e.g. file too small or access error)
		ext := strings.ToLower(filepath.Ext(src))
		switch ext {
		case ".zip":
			format = "zip"
		case ".7z":
			format = "7z"
		case ".rar":
			format = "rar"
		}
	}

	if format == "" {
		return false, nil
	}

	// Try the detected format first
	extracted, err := tryExtract(format, src, destDir)
	if err == nil {
		return extracted, nil
	}

	// If it fails, try other formats as fallback (mislabeled files are common)
	formats := []string{"zip", "7z", "rar"}
	for _, f := range formats {
		if f == format {
			continue
		}
		if ext, err := tryExtract(f, src, destDir); err == nil {
			return ext, nil
		}
	}

	return false, err
}

func sniffFormat(src string) string {
	f, err := os.Open(src)
	if err != nil {
		return ""
	}
	defer f.Close()

	buf := make([]byte, 8)
	n, err := f.Read(buf)
	if err != nil || n < 4 {
		return ""
	}

	// ZIP: PK\x03\x04
	if string(buf[:4]) == "PK\x03\x04" {
		return "zip"
	}
	// 7z: 7z\xbc\xaf\x27\x1c
	if n >= 6 && buf[0] == 0x37 && buf[1] == 0x7A && buf[2] == 0xBC && buf[3] == 0xAF && buf[4] == 0x27 && buf[5] == 0x1C {
		return "7z"
	}
	// RAR: Rar!\x1a\x07
	if n >= 7 && string(buf[:4]) == "Rar!" && buf[4] == 0x1A && buf[5] == 0x07 {
		return "rar"
	}

	return ""
}

func tryExtract(format, src, destDir string) (bool, error) {
	switch format {
	case "zip":
		return extractZip(src, destDir)
	case "7z":
		return extract7z(src, destDir)
	case "rar":
		return extractRar(src, destDir)
	}
	return false, nil
}

func extractZip(src string, destDir string) (bool, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return false, fmt.Errorf("failed to open zip: %w", err)
	}
	defer r.Close()

	hasCue := false
	hasBin := false
	for _, f := range r.File {
		innerExt := strings.ToLower(filepath.Ext(f.Name))
		if innerExt == ".cue" {
			hasCue = true
		} else if innerExt == ".bin" {
			hasBin = true
		}
	}

	if !hasCue || !hasBin {
		return false, nil
	}

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if err := extractFile(f.Name, destDir, func() (io.ReadCloser, error) {
			return f.Open()
		}); err != nil {
			return true, err
		}
	}

	return true, nil
}

func extract7z(src string, destDir string) (bool, error) {
	r, err := sevenzip.OpenReader(src)
	if err != nil {
		return false, fmt.Errorf("failed to open 7z: %w", err)
	}
	defer r.Close()

	hasCue := false
	hasBin := false
	for _, f := range r.File {
		innerExt := strings.ToLower(filepath.Ext(f.Name))
		if innerExt == ".cue" {
			hasCue = true
		} else if innerExt == ".bin" {
			hasBin = true
		}
	}

	if !hasCue || !hasBin {
		return false, nil
	}

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if err := extractFile(f.Name, destDir, func() (io.ReadCloser, error) {
			return f.Open()
		}); err != nil {
			return true, err
		}
	}

	return true, nil
}

func extractRar(src string, destDir string) (bool, error) {
	// First pass: check for .cue and .bin
	f, err := os.Open(src)
	if err != nil {
		return false, fmt.Errorf("failed to open rar: %w", err)
	}
	defer f.Close()

	rr, err := rardecode.NewReader(f)
	if err != nil {
		return false, fmt.Errorf("failed to create rar reader: %w", err)
	}

	hasCue := false
	hasBin := false
	for {
		header, err := rr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, fmt.Errorf("failed to read rar header: %w", err)
		}

		innerExt := strings.ToLower(filepath.Ext(header.Name))
		if innerExt == ".cue" {
			hasCue = true
		} else if innerExt == ".bin" {
			hasBin = true
		}
	}

	if !hasCue || !hasBin {
		return false, nil
	}

	// Second pass: extract everything
	if _, err := f.Seek(0, 0); err != nil {
		return true, fmt.Errorf("failed to seek rar: %w", err)
	}
	rr, err = rardecode.NewReader(f)
	if err != nil {
		return true, fmt.Errorf("failed to recreate rar reader: %w", err)
	}

	for {
		header, err := rr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return true, fmt.Errorf("failed to read rar header during extraction: %w", err)
		}

		if header.IsDir {
			continue
		}

		// rardecode Reader itself is an io.Reader for the current file
		if err := extractFile(header.Name, destDir, func() (io.ReadCloser, error) {
			return io.NopCloser(rr), nil
		}); err != nil {
			return true, err
		}
	}

	return true, nil
}

func extractFile(name string, destDir string, opner func() (io.ReadCloser, error)) error {
	fpath := filepath.Join(destDir, name)
	if err := os.MkdirAll(filepath.Dir(fpath), 0o755); err != nil {
		return err
	}

	rc, err := opner()
	if err != nil {
		return err
	}
	// We don't want to close common readers like rardecode.Reader which we wrap in NopCloser
	// but zip/7z Open returns a ReadCloser that MUST be closed.
	// So we close it if it's not the wrapped NopCloser from RAR.
	defer func() {
		if _, ok := rc.(interface{ Close() error }); ok {
			// Actually NopCloser also has a Close, so we need a better check if possible
			// or just trust the caller. In zip/7z case it's a file reader.
			// In RAR case it's a NopCloser wrapping the main reader.
			// Actually rardecode.Reader doesn't implement Close, but NopCloser does.
			// Let's just close it.
			_ = rc.Close()
		}
	}()

	out, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}
