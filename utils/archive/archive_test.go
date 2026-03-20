package archive

import (
	"archive/zip"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractCueBin_Zip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "archive-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "test.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	zw := zip.NewWriter(f)

	cue, err := zw.Create("game.cue")
	if err != nil {
		t.Fatal(err)
	}
	cue.Write([]byte("FILE \"game.bin\" BINARY\nTRACK 01 MODE1/2352\nINDEX 01 00:00:00"))

	bin, err := zw.Create("game.bin")
	if err != nil {
		t.Fatal(err)
	}
	bin.Write([]byte("fake binary data"))

	zw.Close()
	f.Close()

	destDir := filepath.Join(tmpDir, "extracted")
	extracted, err := ExtractCueBin(zipPath, destDir)
	if err != nil {
		t.Errorf("ExtractCueBin failed: %v", err)
	}
	if !extracted {
		t.Error("expected extracted to be true")
	}

	if _, err := os.Stat(filepath.Join(destDir, "game.cue")); err != nil {
		t.Errorf("game.cue not found in destDir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "game.bin")); err != nil {
		t.Errorf("game.bin not found in destDir: %v", err)
	}
}

func TestExtractCueBin_Mislabeled(t *testing.T) {
	// ZIP file with .rar extension
	tmpDir, err := os.MkdirTemp("", "archive-test-mislabeled-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	rarPath := filepath.Join(tmpDir, "test.rar") // Mislabeled ZIP
	f, err := os.Create(rarPath)
	if err != nil {
		t.Fatal(err)
	}

	zw := zip.NewWriter(f)
	cue, _ := zw.Create("game.cue")
	cue.Write([]byte("fake cue"))
	bin, _ := zw.Create("game.bin")
	bin.Write([]byte("fake bin"))
	zw.Close()
	f.Close()

	destDir := filepath.Join(tmpDir, "extracted")
	extracted, err := ExtractCueBin(rarPath, destDir)
	if err != nil {
		t.Errorf("ExtractCueBin failed for mislabeled file: %v", err)
	}
	if !extracted {
		t.Error("expected extracted to be true for mislabeled file")
	}

	if _, err := os.Stat(filepath.Join(destDir, "game.cue")); err != nil {
		t.Errorf("game.cue not found: %v", err)
	}
}

func TestExtractCueBin_PathTraversal(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "archive-test-traversal-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "test.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	zw := zip.NewWriter(f)
	// Add normal files to satisfy ExtractCueBin check
	c, _ := zw.Create("game.cue")
	c.Write([]byte("fake"))
	b, _ := zw.Create("game.bin")
	b.Write([]byte("fake"))

	// Add malicious file
	m, _ := zw.Create("../../evil.txt")
	m.Write([]byte("evil"))
	zw.Close()
	f.Close()

	destDir := filepath.Join(tmpDir, "extracted")
	_, err = ExtractCueBin(zipPath, destDir)
	if err == nil {
		t.Error("expected error for path traversal, but got none")
	} else if !strings.Contains(err.Error(), "illegal file path") {
		t.Errorf("expected illegal path error, got: %v", err)
	}

	// Verify evil.txt was NOT created outside destDir
	evilPath := filepath.Join(tmpDir, "evil.txt")
	if _, err := os.Stat(evilPath); err == nil {
		t.Error("evil.txt was created outside destination directory!")
	}
}

func TestExtractCueBin_NoPair(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "archive-test-nopair-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, "test.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}

	zw := zip.NewWriter(f)
	c, _ := zw.Create("game.txt") // Only one file, not a pair
	c.Write([]byte("fake"))
	zw.Close()
	f.Close()

	destDir := filepath.Join(tmpDir, "extracted")
	extracted, err := ExtractCueBin(zipPath, destDir)
	if err != nil {
		t.Errorf("ExtractCueBin failed: %v", err)
	}
	if extracted {
		t.Error("expected extracted to be false for archive without .cue/.bin pair")
	}
}

func TestExtractCueBin_7z(t *testing.T) {
	// Minimal 7z file containing game.cue and game.bin
	b64 := "N3q8ryccAAQ7fK9ybgAAAAAAAAAgAAAAAAAAAEKKP6ABAAlmYWtlCmZha2UKAAAAgTMHrg/Ox1yBCSqu06qFJigV5+7h5KJ206s/OLhZ01FnEplGTsr+WPmHbi/IzGUB+fAEJopuBQwond5Pye6K1xROKq7KDsBKRVszsaPyNZhlSFH1GcJqHY80m6AAABcGDgEJYAAHCwEAASMDAQEFXQAQAAAMdgoBluSC3gAA"
	data, _ := base64.StdEncoding.DecodeString(b64)

	tmpDir, err := os.MkdirTemp("", "archive-test-7z-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	szPath := filepath.Join(tmpDir, "test.7z")
	if err := os.WriteFile(szPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	destDir := filepath.Join(tmpDir, "extracted")
	extracted, err := ExtractCueBin(szPath, destDir)
	if err != nil {
		t.Errorf("ExtractCueBin failed for 7z: %v", err)
	}
	if !extracted {
		t.Error("expected extracted to be true for valid 7z")
	}

	if _, err := os.Stat(filepath.Join(destDir, "game.cue")); err != nil {
		t.Errorf("game.cue not found in extracted files: %v", err)
	}
}

func TestExtractCueBin_Rar_Negative(t *testing.T) {
	// Minimal RAR file (from wailsapp/mimetype testdata) - likely doesn't have cue/bin
	b64 := "UmFyIRoHAQAzkrXlCgEFBgAFAQGAgABGzTVJHAICnQEGuwG0gwKAAPNateoMI4ADAQZhc2QuZ2/FBZomVENC9mBE3ZOFQmqQFk3oOpdKCPBUtmRmQWS8HJHNCPelLkzdnFrsv8eqq5PZdHq0VRQffcfxBvgHuPEISMaEcR9TOk1yJwi5Bq4PhPCnZbRiy8PbmvwY8duWLs2WsQlEHekm6N6VH+El5Fuw/U+7eezC+YheVNAtBI5HKV8AkzgXAckVVuswvjQpidDdBz/lkCf1StT9VP6YHXdWUQMFBAA="
	data, _ := base64.StdEncoding.DecodeString(b64)

	tmpDir, err := os.MkdirTemp("", "archive-test-rar-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	rarPath := filepath.Join(tmpDir, "test.rar")
	if err := os.WriteFile(rarPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	destDir := filepath.Join(tmpDir, "extracted")
	extracted, err := ExtractCueBin(rarPath, destDir)
	if err != nil {
		t.Errorf("ExtractCueBin failed for RAR: %v", err)
	}
	if extracted {
		t.Error("expected extracted to be false for RAR without .cue/.bin pair")
	}
}
