package retroarch

import (
	"testing"
)

func TestIdentifyPlatform(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"gb", "gb"},
		{"GB", "gb"},
		{"Nintendo - Game Boy", "gb"},
		{"Game Boy Color", "gbc"},
		{"GBA", "gba"},
		{"Nintendo - Game Boy Advance", "gba"},
		{"Sega CD", "segacd"},
		{"Mega-CD", "segacd"},
		{"3DS", "3ds"},
		{"Nintendo 3DS", "3ds"},
		{"DSi", "dsi"},
		{"Nintendo - DS", "nds"},
		{"GameCube", "gamecube"},
		{"GCN", "gamecube"},
		{"Wii", "wii"},
		{"Sega - Genesis", "genesis"},
		{"Mega Drive", "genesis"},
		{"WonderSwan Color", "wsc"},
		{"WSC", "wsc"},
		{"Neo Geo Pocket Color", "ngp"},
		{"Lynx", "lynx"},
		{"Virtual Boy", "vb"},
		{"Nintendo Entertainment System", "nes"},
		{"NES", "nes"},
		{"Super Nintendo Entertainment System", "snes"},
		{"SNES", "snes"},
		{"PC Engine", "pce"},
		{"TurboGrafx-16", "pce"},
		{"Pokemon Mini", "pokemini"},
		{"Pokémon Mini", "pokemini"},
		{"Pico-8", "pico8"},
		{"Pico 8", "pico8"},
		{"pico8", "pico8"},
		{"p8", "pico8"},
		{"roms", ""},
		{"unknown", ""},
	}

	for _, tt := range tests {
		result := IdentifyPlatform(tt.input)
		if result != tt.expected {
			t.Errorf("IdentifyPlatform(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}
