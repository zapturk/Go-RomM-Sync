# Supported Platforms & Cores

Go-RomM-Sync uses RetroArch (Libretro) cores to launch games. Below is a comprehensive list of supported platforms, their default cores, and compatibility details.

## Emulator Compatibility Table

| Platform | Recommended Core | Extensions | Windows | macOS | Linux | RetroAchievements | Save Sync | Tested |
| :--- | :--- | :--- | :---: | :---: | :---: | :---: | :---: | :---: |
| **3DO** | `opera_libretro` | .iso, .chd | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ |
| **Amstrad CPC** | `caprice32_libretro` | .dsk, .sna | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ |
| **Apple II** | `apple2enh_libretro` | .do, .dsk | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ |
| **Arcade** | `fbneo_libretro` | .zip, .7z | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ |
| **Atari 2600** | `stella_libretro` | .a26 | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ |
| **Atari 5200** | `a5200_libretro` | .a52 | ✅ | ✅ | ✅ | ❌ | ⚠️ | ❌ |
| **Atari 7800** | `prosystem_libretro` | .a78 | ✅ | ✅ | ✅ | ✅ | ⚠️ | ✅ |
| **ColecoVision** | `gearcoleco_libretro` | .col, .rom | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ |
| **Commodore 64** | `vice_x64sc` | .d64, .prg | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ |
| **Dreamcast** | `flycast_libretro` | .gdi, .chd, .cdi, .cue | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ |
| **MSX** | `bluemsx_libretro` | .mx1, .mx2, .rom | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ |
| **Nintendo NES** | `nestopia_libretro` | .nes, .fds | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Nintendo SNES** | `snes9x_libretro` | .sfc, .smc | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Nintendo N64** | `mupen64plus_next` | .z64, .n64, .v64 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Nintendo GB / GBC** | `gambatte_libretro` | .gb, .gbc | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Nintendo GBA** | `mgba_libretro` | .gba | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Nintendo DS / DSi**| `melonds_libretro` | .nds, .dsi | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Nintendo 3DS** | `citra_libretro` | .3ds, .3dsx, .cia | ✅ | ❌ | ✅ | ❌ | ⚠️ | ✅ |
| **Nintendo GameCube**| `dolphin_libretro` | .gcm, .rvz, .iso | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Nintendo Wii** | `dolphin_libretro` | .wbfs, .wia | ✅ | ✅ | ✅ | ❌ | ⚠️ | ❌ |
| **Nintendo Wii U** | `cemu_libretro`* | .wud, .wux, .rpx | ❌ | ❌ | ❌ | ❌ | ⚠️ | ❌ |
| **PC Engine (PCE)** | `mednafen_pce_fast`| .pce, .sgx | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ |
| **Pico-8** | `retro8_libretro` | .p8, .png | ✅ | ✅ | ✅ | ❌ | ⚠️ | ✅ |
| **PlayStation 1** | `pcsx_rearmed` | .iso, .cue, .chd, .bin | ✅ | ✅ | ✅ | ✅ | ⚠️ | ✅ |
| **PlayStation 2** | `pcsx2_libretro` | .iso, .chd | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ |
| **PlayStation Port.**| `ppsspp_libretro` | .iso, .cso | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ |
| **Pokémon Mini** | `pokemini_libretro` | .min | ✅ | ✅ | ✅ | ✅ | ⚠️ | ✅ |
| **Sega 32X** | `picodrive_libretro`| .32x | ✅ | ✅ | ✅ | ✅ | ⚠️ | ✅ |
| **Sega Genesis** | `genesis_plus_gx` | .md, .smd, .gen | ✅ | ✅ | ✅ | ✅ | ⚠️ | ✅ |
| **Sega Master Sys** | `genesis_plus_gx` | .sms, .gg | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ |
| **Sega Saturn** | `mednafen_saturn` | .cue | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ |
| **SG-1000** | `smsplus_libretro` | .sg | ✅ | ✅ | ✅ | ✅ | ⚠️ | ❌ |
| **WonderSwan** | `mednafen_wswan` | .ws, .wsc | ✅ | ✅ | ✅ | ✅ | ⚠️ | ✅ |

*\* Cemu-Libretro currently is having issues with launching games. It is not recommended to use this core at this time.*

## Notes
- **Auto-Download**: The app will attempt to download missing cores from the Libretro buildbot automatically when you click "Play".
- **OS Support**: While most cores are cross-platform, some (like Cemu) have OS-specific limitations.
- **RetroAchievements**: Compatibility depends on both the core and the specific game ROM being recognized by the RetroAchievements database.
- **Save Sync**: The app will attempt to sync saves to RomM. Should be compatible with all cores that support save files but has not been tested on untested cores.
