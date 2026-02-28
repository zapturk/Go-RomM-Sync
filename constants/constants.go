package constants

// OS Names
const (
	OSWindows = "windows"
	OSDarwin  = "darwin"
	OSLinux   = "linux"
)

// Architectures
const (
	ArchAmd64 = "amd64"
	ArchArm64 = "arm64"
	Arch386   = "386"
)

// Event Names
const (
	EventPlayStatus  = "play-status"
	EventGameStarted = "game-started"
	EventGameExited  = "game-exited"
)

// Directory Categories
const (
	DirSaves  = "saves"
	DirStates = "states"
)

// Path Components
const (
	AppDir       = ".go-romm-sync"
	CacheDir     = "cache"
	ConfigDir    = "config"
	CoversDir    = "covers"
	PlatformsDir = "platforms"
)

// Known Cores
const (
	CoreRetro8 = "retro8_libretro"
)
