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
	DirBios   = "bios"
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
	CoreRetro8    = "retro8_libretro"
	CoreCitra     = "citra_libretro"
	CoreAzahar    = "azahar_libretro"
	CoreMelonDS   = "melonds_libretro"
	CoreDeSmuME   = "desmume_libretro"
	CoreMelonDSDS = "melondsds_libretro"
	CoreNooDS     = "noods_libretro"
)

// RomM Scopes
const (
	ScopeMeRead        = "me.read"
	ScopeMeWrite       = "me.write"
	ScopeRomsRead      = "roms.read"
	ScopePlatformsRead = "platforms.read"
	ScopeAssetsRead    = "assets.read"
	ScopeAssetsWrite   = "assets.write"
	ScopeFirmwareRead  = "firmware.read"
	ScopeFirmwareWrite = "firmware.write"
)

// RomMDefaultScopes are the scopes requested for persistent client tokens.
var RomMDefaultScopes = []string{
	ScopeMeRead,
	ScopeRomsRead,
	ScopePlatformsRead,
	ScopeAssetsRead,
	ScopeAssetsWrite,
	ScopeFirmwareRead,
}
