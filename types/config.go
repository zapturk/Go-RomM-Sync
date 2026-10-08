package types

// AppConfig holds all application settings
type AppConfig struct {
	RommHost              string              `json:"romm_host"`            // IP address or url of the RomM server
	Username              string              `json:"username"`             // Username for the RomM server
	Password              string              `json:"password"`             // Password for the RomM server
	LibraryPath           string              `json:"library_path"`         // Where to download ROMs
	RetroArchPath         string              `json:"retroarch_path"`       // Root folder of RA
	RetroArchExecutable   string              `json:"retroarch_executable"` // "retroarch.exe"
	CheevosUsername       string              `json:"cheevos_username"`
	CheevosPassword       string              `json:"cheevos_password"`
	LastUsedCores         map[string]string   `json:"last_used_cores"`         // Platform slug -> Core base name
	PlatformFirmware      map[string]uint     `json:"platform_firmware"`       // Platform slug -> Selected Firmware ID
	OfflineMode           bool                `json:"offline_mode"`            // Enable offline mode
	ClientToken           string              `json:"client_token"`            // Persistent token for the RomM server
	UsePlatformFolder     bool                `json:"use_platform_folder"`     // Store ROMs directly in platform folder
	DisableMetadata       bool                `json:"disable_metadata"`        // Disable metadata.json generation
	GameControllers       map[string]string   `json:"game_controllers"`        // Game ID -> Controller type ID
	GameStartupFiles      map[string]string   `json:"game_startup_files"`      // Game ID -> Selected startup file name
	GameSaveSlots         map[string]string   `json:"game_save_slots"`         // Game ID -> Selected save slot name
	CustomSaveSlots       map[string][]string `json:"custom_save_slots"`       // Game ID -> Custom/local save slots
	ThemeBackground       string              `json:"theme_background"`        // Selected background preset ID or custom mode
	ThemeCustomBackground string              `json:"theme_custom_background"` // Custom color or image path
	ThemeFont             string              `json:"theme_font"`              // Selected font ID
	ThemeTextColor        string              `json:"theme_text_color"`        // Selected font/text color
	ThemeBtnTextColor     string              `json:"theme_btn_text_color"`    // Selected button text color
}

// UIProvider defines standard UI logging and event emission behaviors.
type UIProvider interface {
	LogInfof(format string, args ...interface{})
	LogErrorf(format string, args ...interface{})
	EventsEmit(eventName string, args ...interface{})
}
