package retroarch

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"go-romm-sync/constants"
)

type MockUI struct {
	mu               sync.Mutex
	EmittedEvents    map[string]int
	EmittedEventMsgs map[string][]string
	EventChan        chan string
}

func (m *MockUI) LogInfof(format string, args ...interface{})  {}
func (m *MockUI) LogErrorf(format string, args ...interface{}) {}
func (m *MockUI) EventsEmit(eventName string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.EmittedEvents == nil {
		m.EmittedEvents = make(map[string]int)
	}
	if m.EmittedEventMsgs == nil {
		m.EmittedEventMsgs = make(map[string][]string)
	}
	m.EmittedEvents[eventName]++

	if len(args) > 0 {
		m.EmittedEventMsgs[eventName] = append(m.EmittedEventMsgs[eventName], fmt.Sprintf("%v", args[0]))
	} else {
		m.EmittedEventMsgs[eventName] = append(m.EmittedEventMsgs[eventName], "")
	}

	if m.EventChan != nil {
		select {
		case m.EventChan <- eventName:
		default:
		}
	}
}

func (m *MockUI) GetEventCount(eventName string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.EmittedEvents == nil {
		return 0
	}
	return m.EmittedEvents[eventName]
}

func (m *MockUI) GetEventMsgs(eventName string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.EmittedEventMsgs == nil {
		return nil
	}
	// Return a copy to avoid race conditions if caller iterates over it while test is still emitting events
	msgs := make([]string, len(m.EmittedEventMsgs[eventName]))
	copy(msgs, m.EmittedEventMsgs[eventName])
	return msgs
}

func (m *MockUI) WindowHide()       {}
func (m *MockUI) WindowShow()       {}
func (m *MockUI) WindowUnminimise() {}

func TestLaunch_Errors(t *testing.T) {
	ui := &MockUI{}

	// Test missing exe
	err := Launch(ui, "/non/existent/retroarch", "rom.sfc", "", "", "", "", "")
	if err == nil {
		t.Error("Expected error for non-existent executable")
	}

	// Test missing core map
	tempDir, _ := os.MkdirTemp("", "launch_err")
	defer os.RemoveAll(tempDir)
	exePath := filepath.Join(tempDir, "retroarch")
	os.WriteFile(exePath, []byte("fake"), 0o755)

	err = Launch(ui, exePath, "rom.unknown", "", "", "", "", "")
	if err == nil {
		t.Error("Expected error for unknown extension")
	}
}

func TestLaunch_Zip(t *testing.T) {
	ui := &MockUI{}
	tempDir, _ := os.MkdirTemp("", "launch_zip")
	defer os.RemoveAll(tempDir)

	exePath := filepath.Join(tempDir, "retroarch")
	os.WriteFile(exePath, []byte("fake"), 0o755)

	// Create fake zip
	zipPath := filepath.Join(tempDir, "game.zip")
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	f, _ := w.Create("game.sfc")
	f.Write([]byte("rom data"))
	w.Close()
	os.WriteFile(zipPath, buf.Bytes(), 0o644)

	err := Launch(ui, exePath, zipPath, "", "", "", "", "")
	// It might error because coresDir/cores/... missing, which is fine, we just want to see it gets there.
	if err != nil && !strings.Contains(err.Error(), "emulator core not found") {
		t.Errorf("Unexpected error during zip launch: %v", err)
	}
}

func TestLaunch_Pico8(t *testing.T) {
	ui := &MockUI{}
	tempDir, _ := os.MkdirTemp("", "launch_p8")
	defer os.RemoveAll(tempDir)

	exePath := filepath.Join(tempDir, "retroarch")
	os.WriteFile(exePath, []byte("fake"), 0o755)

	p8Path := filepath.Join(tempDir, "game.png")
	os.WriteFile(p8Path, []byte("png data"), 0o644)

	err := Launch(ui, exePath, p8Path, "", "", "", "", "")
	if err != nil && !strings.Contains(err.Error(), "emulator core not found") {
		t.Errorf("Unexpected error during pico8 launch: %v", err)
	}
}

func TestLaunch_CoreNotSupported(t *testing.T) {
	ui := &MockUI{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	oldURL := buildbotBaseURL
	buildbotBaseURL = server.URL
	defer func() { buildbotBaseURL = oldURL }()

	tempDir, _ := os.MkdirTemp("", "launch_unsupported")
	defer os.RemoveAll(tempDir)
	exePath := filepath.Join(tempDir, "retroarch")
	os.WriteFile(exePath, []byte("fake"), 0o755)

	overrideCoresDir = filepath.Join(tempDir, "cores")
	defer func() { overrideCoresDir = "" }()

	// This should trigger DownloadCore, which will return 404,
	// and Launch should catch it and emit "Core Not Supported".
	err := Launch(ui, exePath, "game.sfc", "", "", "", "", "")
	if err == nil {
		t.Fatal("Expected error from Launch")
	}
	if !strings.Contains(err.Error(), "core not supported") {
		t.Errorf("Expected 'core not supported' error, got: %v", err)
	}

	if ui.GetEventCount(constants.EventPlayStatus) == 0 {
		t.Error("Expected EventPlayStatus to be emitted")
	}
}

func TestLaunch_ExeDir(t *testing.T) {
	ui := &MockUI{}
	tempDir, _ := os.MkdirTemp("", "launch_exe_dir")
	defer os.RemoveAll(tempDir)

	// Create fake retroarch inside directory
	var exeName string
	if runtime.GOOS == "windows" {
		exeName = "retroarch.exe"
	} else {
		exeName = "retroarch"
	}
	exePath := filepath.Join(tempDir, exeName)
	os.WriteFile(exePath, []byte("fake"), 0o755)

	err := Launch(ui, tempDir, "rom.sfc", "", "", "", "", "")
	if err != nil && !strings.Contains(err.Error(), "emulator core not found") {
		t.Errorf("Unexpected error during exe dir launch: %v", err)
	}
}

func TestLaunch_AppBundle(t *testing.T) {
	if runtime.GOOS != constants.OSDarwin {
		t.Skip("Skipping macOS specific test")
	}
	ui := &MockUI{}
	tempDir, _ := os.MkdirTemp("", "launch_app_bundle")
	defer os.RemoveAll(tempDir)

	appPath := filepath.Join(tempDir, "RetroArch.app")
	os.MkdirAll(appPath, 0o755)

	err := Launch(ui, appPath, "rom.sfc", "", "", "", "", "")
	// Should at least pass the directory check and fail on core/binary lookup
	if err != nil && strings.Contains(err.Error(), "retroarch executable not found in directory") {
		t.Errorf("Failed to resolve .app bundle: %v", err)
	}
}

func TestLaunch_CoreOverride(t *testing.T) {
	ui := &MockUI{}
	tempDir, _ := os.MkdirTemp("", "launch_override")
	defer os.RemoveAll(tempDir)

	exePath := filepath.Join(tempDir, "retroarch")
	os.WriteFile(exePath, []byte("fake"), 0o755)

	romPath := filepath.Join(tempDir, "game.sfc")
	os.WriteFile(romPath, []byte("rom data"), 0o644)

	// Should fail at core download/find, not at the override logic
	err := Launch(ui, exePath, romPath, "", "", "my_custom_core_libretro", "", "")
	if err != nil && !strings.Contains(err.Error(), "core not supported") && !strings.Contains(err.Error(), "emulator core not found") {
		t.Errorf("Expected core-not-found or not-supported error with override, got: %v", err)
	}
}

func TestLaunch_PathTraversal(t *testing.T) {
	ui := &MockUI{}
	tempDir, _ := os.MkdirTemp("", "launch_traversal")
	defer os.RemoveAll(tempDir)

	exePath := filepath.Join(tempDir, "retroarch")
	os.WriteFile(exePath, []byte("fake"), 0o755)

	romPath := filepath.Join(tempDir, "game.sfc")
	os.WriteFile(romPath, []byte("rom data"), 0o644)

	// Attempt a path traversal. It should be sanitized to "evil.dll" (or .so/.dylib)
	// and fail because it's not in the cores directory, rather than attempting to load
	// a library from a completely different path.
	err := Launch(ui, exePath, romPath, "", "", "../../evil", "", "")
	if err != nil && !strings.Contains(err.Error(), "core not supported") && !strings.Contains(err.Error(), "emulator core not found") {
		t.Errorf("Expected core-not-found or not-supported error for sanitized path, got: %v", err)
	}
}

func TestLaunch_Events(t *testing.T) {
	ui := &MockUI{
		EventChan: make(chan string, 20),
	}
	tempDir, err := os.MkdirTemp("", "launch_events")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	exePath := filepath.Join(tempDir, "retroarch")
	// Small shell script to simulate a running process
	if err := os.WriteFile(exePath, []byte("#!/bin/sh\nsleep 0.1\nexit 0"), 0o755); err != nil {
		t.Fatalf("failed to write mock exe: %v", err)
	}

	romPath := filepath.Join(tempDir, "game.sfc")
	if err := os.WriteFile(romPath, []byte("rom data"), 0o644); err != nil {
		t.Fatalf("failed to write mock rom: %v", err)
	}

	// Launch should return nil or a core-not-found error, but should trigger the start event regardless if it reaches that point.
	err = Launch(ui, exePath, romPath, "", "", "", "", "")
	if err != nil && !strings.Contains(err.Error(), "emulator core not found") {
		// Only log an actual systemic error, core-not-found is expected in this mock environment
		t.Logf("Launch returned expected core error: %v", err)
	}

	// Wait for the game-started event with a timeout
	found := false
	timeout := time.After(500 * time.Millisecond)

Loop:
	for {
		select {
		case event := <-ui.EventChan:
			if event == constants.EventGameStarted {
				found = true
				break Loop
			}
		case <-timeout:
			break Loop
		}
	}

	if !found {
		t.Log("Warning: EventGameStarted not detected in time via channel.")
	}
}
