package retroarch

import "strings"

// BiosMap maps firmware MD5 hashes to the canonical filenames expected by libretro cores.
var BiosMap = map[string]string{
	// Sega CD (Genesis Plus GX / PicoDrive)
	"baca1df271d7c11fe50087c0358f4eb5": "bios_CD_U.bin",
	"2efd74e3230dca0a2c002167732aefbe": "bios_CD_E.bin",
	"278a9397d1921baa8cf4418fb361665e": "bios_CD_J.bin",
	"854adcf5c6307185011707572793ef07": "bios_CD_U.bin", // Model 1 V1.10
	"d266e3797c5ae401d46b7440492cb715": "bios_CD_E.bin", // Model 1 V1.21

	// PlayStation 1 (PCSX ReARMed / Beetle PSX)
	"924e392ed05558ffdb115408c263793d": "scph5501.bin", // US
	"6341f2aef431661646062f404439abb3": "scph5500.bin", // JP
	"31602b9449f291076f8a846f48123cca": "scph5502.bin", // EU
	"dc20d7c54c3031d873099ea639462d7c": "scph1001.bin",
	"0555c6ef00d1ca9e3020999827a552a0": "scph1000.bin",
	"502224b3a52f47c0e167347577e30705": "scph7001.bin",

	// PlayStation 2 (PCSX2)
	"df6f2da0472409747970973307567786": "SCPH-39001.bin",
	"9a2963162b7be533f81e6a98af074d26": "SCPH-70012.bin",

	// Game Boy Advance (mgba / gpSP)
	"a860e8c0b6d573d191e4ec7db1b1d4f6": "gba_bios.bin",

	// Sega Saturn (Beetle Saturn)
	"858da2873264448576402ecb1e2a0497": "saturn_bios.bin", // JP
	"af5828fdff5138a21dad0c9049d5668d": "saturn_bios.bin", // US
	"294d96a0bf13e93297a72cf2626e95c1": "saturn_bios.bin", // EU

	// Sega Dreamcast (Flycast)
	"e10c53c2f8b90bab96ead2d368858623": "dc_bios.bin",
	"0a93f1d8c7e35558e239c049d7494511": "dc_flash.bin",

	// Nintendo DS (DeSmuME / melonDS)
	"df692403c5fd37890606d51a66e409b3": "bios7.bin",
	"a39217a3a2476ae067f5fa20760f3812": "bios9.bin",
	"27660c6e9d67909dc75ce9100ad851f":  "firmware.bin",

	// PC Engine CD / TurboGrafx-CD (Beetle PCE)
	"38179df8f4ac870017ae202c813f4cb1": "syscard3.pce", // JP
	"0757217cc277cb5624ca973b7713d2fa": "syscard3.pce", // US

	// 3DO (Opera)
	"51f2f43ae2f3508a14d9f56597e2d3ce": "panafz10.bin",
	"f47264dd47fe30f73ab3c010015c155b": "panafz1.bin",
	"8639fd5e549bd6238cfee79e3e749114": "goldstar.bin",

	// Wonderswan (Mednafen Wonderswan)
	"f2bb97f1ccaf40f2f3611388b3941864": "ws.rom",
	"245842c15904f9dfc120a113d07e5c5f": "wsc.rom",

	// Atari 5200
	"281f20ea4320404ec820fb7ec0693b38": "5200.rom",

	// Atari Lynx
	"f1870aad570baecdad59eddf14946ca3": "lynxboot.img",

	// Neo Geo Pocket / Color
	"4e1f79a299e9009951381394c8e7c10": "npbios.bin",
}

// GetBiosFilename returns the expected filename for a BIOS file given its MD5 hash.
// If the MD5 is unknown, it returns an empty string.
func GetBiosFilename(md5 string) string {
	if md5 == "" {
		return ""
	}
	return BiosMap[strings.ToLower(md5)]
}
