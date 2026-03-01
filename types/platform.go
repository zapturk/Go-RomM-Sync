package types

// Platform represents a gaming platform from RomM
type Platform struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	ImageURL string `json:"url_icon"` // Assuming icon/cover URL field
	RomCount int    `json:"rom_count"`
}
