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
}

// FileItem represents a save or state file
type FileItem struct {
	Name string `json:"name"`
	Core string `json:"core"`
}
