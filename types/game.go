package types

// Game represents a ROM/Game from the RomM library
type Game struct {
	ID       uint   `json:"id"`
	Title    string `json:"name"` // API returns "name", we map it to Title
	RomID    uint   `json:"rom_id"`
	CoverURL string `json:"url_cover"`
	FullPath string `json:"full_path"`
}
