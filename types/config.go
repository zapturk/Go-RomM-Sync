package types

// AppConfig holds all application settings
type AppConfig struct {
	RommHost            string `json:"romm_host"`            // IP address or url of the RomM server
	Username            string `json:"username"`             // Username for the RomM server
	Password            string `json:"password"`             // Password for the RomM server
	LibraryPath         string `json:"library_path"`         // Where to download ROMs
	RetroArchPath       string `json:"retroarch_path"`       // Root folder of RA
	RetroArchExecutable string `json:"retroarch_executable"` // "retroarch.exe"
}
