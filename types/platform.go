package types

// Platform represents a gaming platform from RomM
type Platform struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	ImageURL string `json:"url_icon"` // Assuming icon/cover URL field
	RomCount int    `json:"rom_count"`
}

// Firmware represents a BIOS or firmware file from RomM
type Firmware struct {
	ID         uint   `json:"id"`
	FileName   string `json:"file_name"`
	FileSize   int64  `json:"file_size_bytes"`
	IsVerified bool   `json:"is_verified"`
	PlatformID uint   `json:"platform_id"`
	MD5Hash    string `json:"md5_hash"`
}
