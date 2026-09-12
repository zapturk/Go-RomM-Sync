package retroarch

import (
	"runtime"
	"strings"
	"testing"

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

