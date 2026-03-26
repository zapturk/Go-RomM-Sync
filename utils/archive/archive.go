package archive

import (
	"archive/zip"
	"bytes"
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

	extCue = ".cue"
	extBin = ".bin"
)

// ZipDirToBuffer zips a directory and returns its content as a byte slice.
func ZipDirToBuffer(dirPath string) ([]byte, error) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}
		f, err := zw.Create(relPath)
		if err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = src.Close() }()
		_, err = io.Copy(f, src)
		return err
	})

	if err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Extract extracts all files from an archive to the destination directory.
// Returns true if files were extracted, false if not a recognized archive.
func Extract(src, destDir string) (bool, error) {
	format := sniffFormat(src)
	if format == "" {
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

	return tryExtract(format, src, destDir, false)
}

// ExtractCueBin checks if an archive contains .cue and .bin files and extracts them if it does.
// Returns true if files were extracted, false if not an archive or no .cue/.bin pair found.
func ExtractCueBin(src, destDir string) (bool, error) {
	return extractByCondition(src, destDir, func(entries []archiveEntry) bool {
		hasCue := false
		hasBin := false
		for _, e := range entries {
			switch strings.ToLower(filepath.Ext(e.Name())) {
			case extCue:
				hasCue = true
			case extBin:
				hasBin = true
			}
		}
		return hasCue && hasBin
	})
}

// ExtractGameCube checks if an archive contains .rvz, .gcm, or .gcz files and extracts them if it does.
// Returns true if files were extracted, false if not an archive or no GameCube ROM found.
func ExtractGameCube(src, destDir string) (bool, error) {
	return extractByCondition(src, destDir, func(entries []archiveEntry) bool {
		for _, e := range entries {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext == ".rvz" || ext == ".gcm" || ext == ".gcz" {
				return true
			}
		}
		return false
	})
}

// ExtractPS2 checks if an archive contains .iso, .chd, or .cso files and extracts them if it does.
// Returns true if files were extracted, false if not an archive or no PS2 ROM found.
func ExtractPS2(src, destDir string) (bool, error) {
	return extractByCondition(src, destDir, func(entries []archiveEntry) bool {
		for _, e := range entries {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext == ".iso" || ext == ".chd" || ext == ".cso" {
				return true
			}
		}
		return false
	})
}

func extractByCondition(src, destDir string, condition func([]archiveEntry) bool) (bool, error) {
	format := sniffFormat(src)
	if format == "" {
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

	// Try extracting with the condition
	return tryExtractWithCondition(format, src, destDir, condition)
}

func tryExtractWithCondition(format, src, destDir string, condition func([]archiveEntry) bool) (bool, error) {
	switch format {
	case formatZip:
		return extractZipWithCondition(src, destDir, condition)
	case format7z:
		return extract7zWithCondition(src, destDir, condition)
	case formatRar:
		return extractRarWithCondition(src, destDir, condition)
	}
	return false, nil
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

func tryExtract(format, src, destDir string, cueBinOnly bool) (bool, error) {
	if cueBinOnly {
		return ExtractCueBin(src, destDir)
	}
	return tryExtractWithCondition(format, src, destDir, nil)
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

func extractZipWithCondition(src, destDir string, condition func([]archiveEntry) bool) (bool, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return false, fmt.Errorf("failed to open zip: %w", err)
	}
	defer func() { _ = r.Close() }()

	entries := make([]archiveEntry, len(r.File))
	for i, f := range r.File {
		entries[i] = zipEntry{f}
	}

	if condition != nil && !condition(entries) {
		return false, nil
	}

	return processArchiveEntries(entries, destDir, false)
}

func extract7zWithCondition(src, destDir string, condition func([]archiveEntry) bool) (bool, error) {
	r, err := sevenzip.OpenReader(src)
	if err != nil {
		return false, fmt.Errorf("failed to open 7z: %w", err)
	}
	defer func() { _ = r.Close() }()

	entries := make([]archiveEntry, len(r.File))
	for i, f := range r.File {
		entries[i] = sevenZipEntry{f}
	}

	if condition != nil && !condition(entries) {
		return false, nil
	}

	return processArchiveEntries(entries, destDir, false)
}

func processArchiveEntries(entries []archiveEntry, destDir string, cueBinOnly bool) (bool, error) {
	if cueBinOnly {
		hasCue := false
		hasBin := false
		for _, e := range entries {
			switch strings.ToLower(filepath.Ext(e.Name())) {
			case extCue:
				hasCue = true
			case extBin:
				hasBin = true
			}
		}

		if !hasCue || !hasBin {
			return false, nil
		}
	}

	if len(entries) == 0 {
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

func extractRarWithCondition(src, destDir string, condition func([]archiveEntry) bool) (bool, error) {
	f, err := os.Open(src)
	if err != nil {
		return false, fmt.Errorf("failed to open rar: %w", err)
	}
	defer func() { _ = f.Close() }()

	rr, err := rardecode.NewReader(f)
	if err != nil {
		return false, fmt.Errorf("failed to create rar reader: %w", err)
	}

	// Since rardecode reader doesn't support random access/peeking without consuming,
	// we have to check the condition by iterating, then re-open if we need to extract.
	// This is inefficient but necessary for RAR.
	if condition != nil {
		var entries []archiveEntry
		for {
			header, err := rr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return false, err
			}
			entries = append(entries, rarEntry{header})
		}
		if !condition(entries) {
			return false, nil
		}

		// Re-open
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return true, err
		}
		rr, err = rardecode.NewReader(f)
		if err != nil {
			return true, err
		}
	}

	err = performRarExtraction(rr, destDir)
	return true, err
}

type rarEntry struct {
	*rardecode.FileHeader
}

func (e rarEntry) Name() string { return e.FileHeader.Name }
func (e rarEntry) IsDir() bool  { return e.FileHeader.IsDir }
func (e rarEntry) Open() (io.ReadCloser, error) {
	return nil, fmt.Errorf("rar entry open not implemented for condition check")
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
