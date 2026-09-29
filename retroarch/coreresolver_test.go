package retroarch

import (
	"archive/zip"
	"go-romm-sync/types"
	"os"
	"path/filepath"
	"testing"
)

type mockLibProvider struct {
	romDir string
}

func (m *mockLibProvider) GetRomDir(game *types.Game) string {
	return m.romDir
}

func TestCoreResolver_GameCube(t *testing.T) {
	resolver := NewCoreResolver(nil)

	// Strategy 1: PlatformSlug "ngc"
	cores := resolver.Resolve(ResolveOptions{
		PlatformSlug: "ngc",
	})
	if len(cores) == 0 || cores[0] != "dolphin_libretro" {
		t.Fatalf("expected dolphin_libretro for slug ngc, got: %v", cores)
	}

	// Strategy 1: PlatformSlug "gamecube"
	cores = resolver.Resolve(ResolveOptions{
		PlatformSlug: "gamecube",
	})
	if len(cores) == 0 || cores[0] != "dolphin_libretro" {
		t.Fatalf("expected dolphin_libretro for slug gamecube, got: %v", cores)
	}

	// Strategy 2: Path containing "ngc"
	cores = resolver.Resolve(ResolveOptions{
		FullPath: "roms/ngc/game.zip",
	})
	if len(cores) == 0 || cores[0] != "dolphin_libretro" {
		t.Fatalf("expected dolphin_libretro for path with ngc, got: %v", cores)
	}

	// Strategy 3: Extension .rvz
	cores = resolver.Resolve(ResolveOptions{
		FullPath: "unknown/game.rvz",
	})
	if len(cores) == 0 || cores[0] != "dolphin_libretro" {
		t.Fatalf("expected dolphin_libretro for .rvz extension, got: %v", cores)
	}
}

func TestCoreResolver_LocalFileScan_GameCubeZip(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "coreresolver_gc_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	zipPath := filepath.Join(tempDir, "game.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	w, err := zw.Create("game.rvz")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("fake rvz"))
	_ = zw.Close()
	_ = zf.Close()

	mockLib := &mockLibProvider{romDir: tempDir}
	resolver := NewCoreResolver(mockLib)

	// Local file scan when platform slug and extension are ambiguous
	cores := resolver.Resolve(ResolveOptions{
		GameID:   123,
		FullPath: "unknown/game.zip",
	})
	if len(cores) == 0 || cores[0] != "dolphin_libretro" {
		t.Fatalf("expected dolphin_libretro from local zip peek, got: %v", cores)
	}
}
