package retroarch

import (
	"runtime"
	"strings"
	"testing"

	"github.com/bodgit/sevenzip"

	"go-romm-sync/constants"
)

func TestGetRetroArchDownloadURL(t *testing.T) {
	version := "1.22.2"

	tests := []struct {
		goos        string
		goarch      string
		expectedURL string
		expectError bool
	}{
		{
			goos:        constants.OSDarwin,
			goarch:      constants.ArchArm64,
			expectedURL: "https://buildbot.libretro.com/stable/1.22.2/apple/osx/universal/RetroArch_Metal.dmg",
			expectError: false,
		},
		{
			goos:        constants.OSDarwin,
			goarch:      constants.ArchAmd64,
			expectedURL: "https://buildbot.libretro.com/stable/1.22.2/apple/osx/universal/RetroArch_Metal.dmg",
			expectError: false,
		},
		{
			goos:        constants.OSWindows,
			goarch:      constants.ArchAmd64,
			expectedURL: "https://buildbot.libretro.com/stable/1.22.2/windows/x86_64/RetroArch.7z",
			expectError: false,
		},
		{
			goos:        constants.OSWindows,
			goarch:      constants.Arch386,
			expectedURL: "https://buildbot.libretro.com/stable/1.22.2/windows/x86/RetroArch.7z",
			expectError: false,
		},
		{
			goos:        constants.OSLinux,
			goarch:      constants.ArchAmd64,
			expectedURL: "https://buildbot.libretro.com/stable/1.22.2/linux/x86_64/RetroArch.7z",
			expectError: false,
		},
		{
			goos:        constants.OSLinux,
			goarch:      constants.Arch386,
			expectedURL: "https://buildbot.libretro.com/stable/1.22.2/linux/x86/RetroArch.7z",
			expectError: false,
		},
		{
			goos:        "freebsd",
			goarch:      constants.ArchAmd64,
			expectedURL: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		url, err := getRetroArchDownloadURL(version, tt.goos, tt.goarch)
		if tt.expectError {
			if err == nil {
				t.Errorf("expected error for %s/%s, got url: %s", tt.goos, tt.goarch, url)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error for %s/%s: %v", tt.goos, tt.goarch, err)
			}
			if url != tt.expectedURL {
				t.Errorf("getRetroArchDownloadURL(%s, %s) = %q; expected %q", tt.goos, tt.goarch, url, tt.expectedURL)
			}
		}
	}
}

func TestGetDefaultInstallDir(t *testing.T) {
	dir, err := getDefaultInstallDir(runtime.GOOS)
	if err != nil {
		t.Fatalf("getDefaultInstallDir failed for current OS (%s): %v", runtime.GOOS, err)
	}
	if dir == "" {
		t.Errorf("expected non-empty install dir for current OS (%s)", runtime.GOOS)
	}

	// Test cross-OS logic
	winDir, err := getDefaultInstallDir(constants.OSWindows)
	if err != nil || !strings.Contains(winDir, "RetroArch") {
		t.Errorf("expected Windows install dir to contain 'RetroArch', got %s, err: %v", winDir, err)
	}

	linuxDir, err := getDefaultInstallDir(constants.OSLinux)
	if err != nil || !strings.Contains(linuxDir, ".local") {
		t.Errorf("expected Linux install dir to contain '.local', got %s, err: %v", linuxDir, err)
	}
}

func TestIsExecutableEntry(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"retroarch.exe", true},
		{"RetroArch.AppImage", true},
		{"retroarch", true},
		{"bin/tool", true},
		{"assets/image.png", false},
		{"retroarch.cfg", false},
	}

	for _, tt := range tests {
		if got := isExecutableEntry(tt.name); got != tt.expected {
			t.Errorf("isExecutableEntry(%q) = %v, want %v", tt.name, got, tt.expected)
		}
	}
}

func TestDetect7zCommonPrefix(t *testing.T) {
	tests := []struct {
		names    []string
		expected string
	}{
		{
			names:    []string{"RetroArch-Win64/retroarch.exe", "RetroArch-Win64/retroarch.cfg"},
			expected: "RetroArch-Win64/",
		},
		{
			names:    []string{"retroarch.exe", "retroarch.cfg"},
			expected: "",
		},
		{
			names:    []string{"RetroArch-Win64/retroarch.exe", "OtherDir/retroarch.cfg"},
			expected: "",
		},
		{
			names:    nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		var files []*sevenzip.File
		for _, name := range tt.names {
			f := &sevenzip.File{
				FileHeader: sevenzip.FileHeader{
					Name: name,
				},
			}
			files = append(files, f)
		}
		if got := detect7zCommonPrefix(files); got != tt.expected {
			t.Errorf("detect7zCommonPrefix(%v) = %q, want %q", tt.names, got, tt.expected)
		}
	}
}

func TestDetermine7zFileMode(t *testing.T) {
	execFile := &sevenzip.File{
		FileHeader: sevenzip.FileHeader{
			Name: "retroarch.exe",
		},
	}
	// Executable entry gets 0755
	if mode := determine7zFileMode(execFile, "retroarch.exe"); mode.Perm() != 0o755 {
		t.Errorf("expected 0755 for executable, got %v", mode)
	}

	regularFile := &sevenzip.File{
		FileHeader: sevenzip.FileHeader{
			Name: "assets/font.ttf",
		},
	}
	if mode := determine7zFileMode(regularFile, "assets/font.ttf"); mode.Perm() != 0o666 && mode.Perm() != 0o644 {
		t.Errorf("expected 0666 or 0644 for regular file, got %v", mode)
	}
}
