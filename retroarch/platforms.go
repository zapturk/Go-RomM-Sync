package retroarch

import (
	"go-romm-sync/constants"
	"regexp"
	"strings"
)

var dsRegex = regexp.MustCompile(`\bds\b`)

// PlatformCoreMap maps common platform names or slugs to an ordered list
// of known-working libretro core base names.
var PlatformCoreMap = map[string][]string{
	"gb":           {"gambatte_libretro", "mgba_libretro", "sameboy_libretro"},
	"gbc":          {"gambatte_libretro", "mgba_libretro", "sameboy_libretro"},
	"gba":          {"mgba_libretro", "vba_next_libretro"},
	"nes":          {"nestopia_libretro", "fceumm_libretro", "mesen_libretro"},
	"snes":         {"snes9x_libretro", "bsnes_libretro"},
	"n64":          {"mupen64plus_next_libretro", "parallel_n64_libretro"},
	"nds":          {constants.CoreMelonDSDS, constants.CoreNooDS, constants.CoreMelonDS, constants.CoreDeSmuME},
	"dsi":          {constants.CoreMelonDSDS, constants.CoreNooDS, constants.CoreMelonDS, constants.CoreDeSmuME},
	"genesis":      {"genesis_plus_gx_libretro", "picodrive_libretro", "blastem_libretro"},
	"mastersystem": {"genesis_plus_gx_libretro", "picodrive_libretro"},
	"gamegear":     {"genesis_plus_gx_libretro"},
	"ps1":          {"pcsx_rearmed_libretro", "beetle_psx_libretro"},
	"psp":          {"ppsspp_libretro"},
	"dreamcast":    {"flycast_libretro"},
	"pce":          {"mednafen_pce_fast_libretro", "mednafen_pce_libretro"},
	"gamecube":     {"dolphin_libretro"},
	"wii":          {"dolphin_libretro"},
	"3ds":          {constants.CoreAzahar, constants.CoreCitra},
	"pico8":        {"retro8_libretro"},
	"wsc":          {"mednafen_wswan_libretro"},
	"ngp":          {"mednafen_ngp_libretro"},
	"vb":           {"beetle_vb_libretro"},
	"lynx":         {"handy_libretro"},
	"pce_fast":     {"mednafen_pce_fast_libretro"},
	"supergrafx":   {"mednafen_pce_fast_libretro"},
	"a78":          {"prosystem_libretro"},
	"3do":          {"opera_libretro"},
	"amstrad":      {"caprice32_libretro"},
	"apple2":       {"apple2enh_libretro"},
	"arcade":       {"fbneo_libretro", "mame2003_plus_libretro"},
	"coleco":       {"gearcoleco_libretro"},
	"msx":          {"bluemsx_libretro"},
	"ps2":          {"pcsx2_libretro", "play_libretro"},
	"sg1000":       {"smsplus_libretro"},
	"neogeo":       {"fbneo_libretro"},
	"a26":          {"stella_libretro"},
	"a52":          {"a5200_libretro"},
	"c64":          {"vice_x64sc_libretro"},
	"32x":          {"picodrive_libretro"},
	"saturn":       {"mednafen_saturn_libretro"},
	"wiiu":         {"cemu_libretro"},
	"segacd":       {"genesis_plus_gx_libretro", "picodrive_libretro"},
	"pokemini":     {"pokemini_libretro"},
}

// GetCoresForPlatform returns the ordered list of known-working libretro core
// base-names for the given platform slug or name.
func GetCoresForPlatform(platform string) []string {
	if platform == "" {
		return nil
	}
	// Try the direct mapping first.
	if cores, ok := PlatformCoreMap[strings.ToLower(platform)]; ok {
		return cores
	}
	// Fallback to fuzzy identification.
	slug := IdentifyPlatform(platform)
	if slug != "" {
		return PlatformCoreMap[slug]
	}
	return nil
}

// platformSearchPatterns defines fuzzy matching rules for identifying platforms from strings.
// Order matters: more specific patterns (e.g. "snes") should come before more general ones (e.g. "nes").
var platformSearchPatterns = []struct {
	slug     string
	patterns []string
	all      bool
}{
	// Consoles - Specific/Modern first to avoid broad matches
	{"wiiu", []string{"wii u", "wiiu"}, false},
	{"wii", []string{"wii"}, false},
	{"gamecube", []string{"gamecube", "gcn", "dolphin"}, false},
	{"n64", []string{"n64", "nintendo 64"}, false},
	{"ps2", []string{"ps2", "playstation 2"}, false},
	{"ps1", []string{"playstation", "ps1", "psx"}, false},
	{"dreamcast", []string{"dreamcast"}, false},
	{"saturn", []string{"saturn"}, false},
	{"genesis", []string{"genesis", "mega drive", "megadrive"}, false},
	{"snes", []string{"snes", "super nintendo", "super entertainment system"}, false},
	{"nes", []string{"nes", "entertainment system"}, false},
	{"mastersystem", []string{"master system", "mastersystem"}, false},
	{"segacd", []string{"sega cd", "segacd", "mega-cd", "megacd"}, false},
	{"pce", []string{"pce", "pc engine", "turbo", "grafx"}, false},
	{"3do", []string{"3do"}, false},

	// Handhelds
	{"gba", []string{"advance", "gba"}, false},
	{"dsi", []string{"dsi"}, false},
	{"3ds", []string{"3ds"}, false},
	{"nds", []string{"nintendo ds", "nds", "dual screen", "ds"}, false},
	{"psp", []string{"psp", "playstation portable"}, false},
	{"wsc", []string{"wonderswan", "wsc"}, false},
	{"ngp", []string{"neo", "pocket"}, true},
	{"vb", []string{"virtual", "boy"}, true},
	{"lynx", []string{"lynx"}, false},
	{"pico8", []string{"pico-8", "pico8", "pico 8", "p8"}, false},
	{"gamegear", []string{"game gear", "gamegear"}, false},
	{"gbc", []string{"color", "gbc"}, false},
	{"gb", []string{"game boy", "gb"}, false},

	// 8-bit / Classic
	{"a78", []string{"7800"}, false},
	{"a52", []string{"5200"}, false},
	{"a26", []string{"2600"}, false},
	{"32x", []string{"32x"}, false},
	{"sg1000", []string{"sg1000", "sg-1000"}, false},
	{"coleco", []string{"coleco"}, false},

	// Computers
	{"c64", []string{"c64", "commodore"}, false},
	{"msx", []string{"msx"}, false},
	{"amstrad", []string{"amstrad", "cpc"}, false},
	{"apple2", []string{"apple", "ii"}, true},

	// Others
	{"arcade", []string{"arcade", "mame", "fbneo"}, false},
	{"neogeo", []string{"neo geo", "neogeo"}, false},
	{"pokemini", []string{"pokemini", "pokemon mini", "pokémon mini", "pokemonmini", "pokémonmini"}, false},
}

// IdentifyPlatform attempts to resolve a canonical platform slug from a string,
// such as a folder name or a tag (e.g., "Nintendo - Game Boy" -> "gb").
func IdentifyPlatform(input string) string {
	lower := strings.ToLower(input)
	if lower == "" || lower == "roms" {
		return ""
	}

	// 0. Extension-based hint (High Priority)
	// If the input contains a known GameCube/Wii extension, prioritize it.
	if strings.Contains(lower, ".rvz") || strings.Contains(lower, ".gcz") || strings.Contains(lower, ".gcm") {
		return "gamecube"
	}
	if strings.Contains(lower, ".wbfs") {
		return "wii"
	}

	// 1. Direct check for exact slug matches (Primary)
	if _, ok := PlatformCoreMap[lower]; ok {
		return lower
	}

	// 2. Fuzzy matching based on search patterns
	for _, entry := range platformSearchPatterns {
		if matchPattern(entry, lower) {
			return entry.slug
		}
	}
	return ""
}

func matchPattern(entry struct {
	slug     string
	patterns []string
	all      bool
}, lower string) bool {
	if entry.all {
		for _, p := range entry.patterns {
			if !strings.Contains(lower, p) {
				return false
			}
		}
		return true
	}

	for _, p := range entry.patterns {
		// Special check for "ds" to ensure it's a word, not a substring
		if p == "ds" {
			if dsRegex.MatchString(lower) {
				return true
			}
			continue
		}

		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
