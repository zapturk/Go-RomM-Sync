package types

// Game represents a ROM/Game from the RomM library
type Game struct {
	ID       uint     `json:"id"`
	Title    string   `json:"name"` // API returns "name", we map it to Title
	RomID    uint     `json:"rom_id"`
	CoverURL string   `json:"url_cover"`
	FullPath string   `json:"full_path"`
	Summary  string   `json:"summary"`
	Genres   []string `json:"genres"`
	HasSaves bool     `json:"has_saves"` // Simplified for now, though API might return a list
	FileSize int64    `json:"fs_size_bytes"`
}

// FileItem represents a local save or state file
type FileItem struct {
	Name      string `json:"name"`
	Core      string `json:"core"`
	UpdatedAt string `json:"updated_at"` // ISO8601 string
}

// ServerSave represents a save file on the RomM server
type ServerSave struct {
	ID        uint   `json:"id"`
	FileName  string `json:"file_name"`
	FullPath  string `json:"full_path"`
	Emulator  string `json:"emulator"`
	UpdatedAt string `json:"updated_at"` // ISO8601 string
	FileSize  int64  `json:"file_size_bytes"`
}

// ServerState represents a save state on the RomM server
type ServerState struct {
	ID        uint   `json:"id"`
	FileName  string `json:"file_name"`
	FullPath  string `json:"full_path"`
	Emulator  string `json:"emulator"`
	UpdatedAt string `json:"updated_at"` // ISO8601 string
	FileSize  int64  `json:"file_size_bytes"`
}
