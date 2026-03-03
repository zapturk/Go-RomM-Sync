package archive

import (
	"archive/zip"
	"os"
	"path/filepath"
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
