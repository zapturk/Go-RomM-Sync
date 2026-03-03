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

const (
	formatZip = "zip"
	format7z  = "7z"
	formatRar = "rar"
)

// ExtractCueBin checks if an archive contains .cue and .bin files and extracts them if it does.
// Returns true if files were extracted, false if not an archive or no .cue/.bin pair found.
func ExtractCueBin(src, destDir string) (bool, error) {
	format := sniffFormat(src)
	if format == "" {
		// Fallback to extension if sniffing fails (e.g. file too small or access error)
		ext := strings.ToLower(filepath.Ext(src))
		switch ext {
		case ".zip":
			format = formatZip
		case ".7z":
			format = format7z
		case ".rar":
			format = formatRar
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
	formats := []string{formatZip, format7z, formatRar}
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
	defer func() { _ = f.Close() }()

	buf := make([]byte, 8)
	n, err := f.Read(buf)
	if err != nil || n < 4 {
		return ""
	}

	if sn := sniffZip(buf); sn != "" {
		return sn
	}
	if sn := sniff7z(buf, n); sn != "" {
		return sn
	}
	if sn := sniffRar(buf, n); sn != "" {
		return sn
	}

	return ""
}

func sniffZip(buf []byte) string {
	if string(buf[:4]) == "PK\x03\x04" {
		return formatZip
	}
	return ""
}

func sniff7z(buf []byte, n int) string {
	if n >= 6 && buf[0] == 0x37 && buf[1] == 0x7A && buf[2] == 0xBC && buf[3] == 0xAF && buf[4] == 0x27 && buf[5] == 0x1C {
		return format7z
	}
	return ""
}

func sniffRar(buf []byte, n int) string {
	if n >= 7 && string(buf[:4]) == "Rar!" && buf[4] == 0x1A && buf[5] == 0x07 {
		return formatRar
	}
	return ""
}

func tryExtract(format, src, destDir string) (bool, error) {
	switch format {
	case formatZip:
		return extractZip(src, destDir)
	case format7z:
		return extract7z(src, destDir)
	case formatRar:
		return extractRar(src, destDir)
	}
	return false, nil
}

type archiveEntry interface {
	Name() string
	IsDir() bool
	Open() (io.ReadCloser, error)
}

type zipEntry struct {
	*zip.File
}

func (e zipEntry) Name() string                 { return e.File.Name }
func (e zipEntry) IsDir() bool                  { return e.File.FileInfo().IsDir() }
func (e zipEntry) Open() (io.ReadCloser, error) { return e.File.Open() }

type sevenZipEntry struct {
	*sevenzip.File
}

func (e sevenZipEntry) Name() string                 { return e.File.Name }
func (e sevenZipEntry) IsDir() bool                  { return e.File.FileInfo().IsDir() }
func (e sevenZipEntry) Open() (io.ReadCloser, error) { return e.File.Open() }

func extractZip(src, destDir string) (bool, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return false, fmt.Errorf("failed to open zip: %w", err)
	}
	defer func() { _ = r.Close() }()

	entries := make([]archiveEntry, len(r.File))
	for i, f := range r.File {
		entries[i] = zipEntry{f}
	}

	return processArchiveEntries(entries, destDir)
}

func extract7z(src, destDir string) (bool, error) {
	r, err := sevenzip.OpenReader(src)
	if err != nil {
		return false, fmt.Errorf("failed to open 7z: %w", err)
	}
	defer func() { _ = r.Close() }()

	entries := make([]archiveEntry, len(r.File))
	for i, f := range r.File {
		entries[i] = sevenZipEntry{f}
	}

	return processArchiveEntries(entries, destDir)
}

func processArchiveEntries(entries []archiveEntry, destDir string) (bool, error) {
	hasCue := false
	hasBin := false
	for _, e := range entries {
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".cue":
			hasCue = true
		case ".bin":
			hasBin = true
		}
	}

	if !hasCue || !hasBin {
		return false, nil
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := extractFile(e.Name(), destDir, e.Open); err != nil {
			return true, err
		}
	}

	return true, nil
}

func extractRar(src, destDir string) (bool, error) {
	// First pass: check for .cue and .bin
	f, err := os.Open(src)
	if err != nil {
		return false, fmt.Errorf("failed to open rar: %w", err)
	}
	defer func() { _ = f.Close() }()

	rr, err := rardecode.NewReader(f)
	if err != nil {
		return false, fmt.Errorf("failed to create rar reader: %w", err)
	}

	if hasCue, hasBin, err := checkRarCueBin(rr); err != nil {
		return false, err
	} else if !hasCue || !hasBin {
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

	return true, performRarExtraction(rr, destDir)
}

func checkRarCueBin(rr *rardecode.Reader) (hasCue, hasBin bool, err error) {
	hasCue = false
	hasBin = false
	for {
		header, err := rr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, false, fmt.Errorf("failed to read rar header: %w", err)
		}

		switch strings.ToLower(filepath.Ext(header.Name)) {
		case ".cue":
			hasCue = true
		case ".bin":
			hasBin = true
		}
	}
	return hasCue, hasBin, nil
}

func performRarExtraction(rr *rardecode.Reader, destDir string) error {
	for {
		header, err := rr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read rar header during extraction: %w", err)
		}

		if header.IsDir {
			continue
		}

		// rardecode Reader itself is an io.Reader for the current file
		if err := extractFile(header.Name, destDir, func() (io.ReadCloser, error) {
			return io.NopCloser(rr), nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(name, destDir string, opner func() (io.ReadCloser, error)) error {
	fpath := filepath.Join(destDir, name)

	// Path traversal protection: ensure the resolved path is within destDir
	destDirClean := filepath.Clean(destDir) + string(os.PathSeparator)
	if !strings.HasPrefix(filepath.Clean(fpath), destDirClean) {
		return fmt.Errorf("illegal file path in archive: %s (traversal attempt)", name)
	}

	if err := os.MkdirAll(filepath.Dir(fpath), 0o755); err != nil {
		return err
	}

	rc, err := opner()
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()

	out, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, rc)
	return err
}
