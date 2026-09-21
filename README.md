# Cosmetic Editor

A Go/Fyne app for editing Echo VR cosmetics: tints, emotes, emissives, banners, tags, medals and their metadata. It runs on Windows for PC VR and **on the Quest itself** as an Android app. Built by [he_is_the_cat](https://github.com/heisthecat31), based on [goopsie](https://github.com/goopsie)'s reverse engineering work and `evrFileTools`.

## What It Does

The Cosmetic Editor lets you modify everything about a cosmetic item: textures, colours, animations, display names, descriptions and rarity. Edits are autosaved as you make them, and **Repack & Apply** writes them into the game's package files.

### vs. Texture Editor

The [Texture Editor](https://github.com/EchoTools/EchoVR-Texture-Editor) (goopsie's Python tool) gives you access to all ~12,000 individual textures in the game data, but is limited to raw texture replacement: you can swap a tag's image, but can't touch its metadata, name or rarity.

The Cosmetic Editor is narrower in scope (cosmetics only, not all textures) but deeper, with full control over every cosmetic property. Both tools use the same folder layout (`Settings/texture_cache/`) and can be used side by side.

| | Texture Editor | Cosmetic Editor |
|---|---|---|
| Scope | All ~12k textures | Cosmetic items only |
| Language | Python (single script) | Go (Fyne GUI) |
| Model import | No | Yes, PC only (chassis, bracers, boosters from .blend/.glb via Blender) |
| Metadata editing | No | Yes (names, descriptions, rarity) |
| Tints | No | Yes (hex or colour picker, thumbnail generation) |
| Emissives | No | Yes (gradients, scroll presets) |
| Emotes | No | Yes (GIF import, frame slicing, export) |
| Runs on Quest | No (pushes over ADB) | Yes (Android APK, edits and repacks on the headset) |
| Extraction/repacking | Yes (via evrFileTools) | Repacking built in; no extraction needed |

### Cosmetic Categories

- **Tints**: primary and secondary colours. Type a hex value or use the colour picker; invalid input is filtered out as you type. **Generate Thumbnail** recolours the tint template with the tint's exact colours.
- **Emotes**: GIF-to-emote conversion. Import a GIF and it is sliced into frames, encoded and repacked. Existing emotes can be exported as GIFs.
- **Emissives**: scrolling texture presets and multi-colour gradients.
- **Banners, Tags, Emblems**: high-res texture replacement.
- **Medals, Pips, Patterns, Titles, Decals, Fanfares**: full metadata and texture editing. Fanfare audio is PC only.
- **Metadata**: display names, internal names, descriptions.
- **Rarity**: Mythic, Legendary, Epic, Superb, Fine, Common, Default.

- **Chassis, Bracers, Boosters** (PC only): metadata editing, plus **Replace Model from .blend** to import a custom model from a `.blend` or `.glb` (chassis: 1st person, 3rd person or both; bracers: left, right or both). This needs Python and Blender installed. The game's original mesh is read straight from the package as the base, so nothing has to be extracted. In Quest mode these tabs are hidden.

## How It Works

1. **Find the game data.** On a Quest the app finds it by itself (see below). On PC, pick your Echo VR folder: the game folder, `_data`, or anything in between works, and the app finds the `rad15/win10` data directory beneath it.
2. **Previews** are decoded straight from the game's package files, in the background, and cached in `Settings/texture_cache/`. Nothing has to be extracted first.
3. **Edit** cosmetics. Changes are autosaved to disk as you make them, so they survive closing the app.
4. **Repack & Apply** stages the cosmetic database, replaced textures and installed mods, then appends them to the game's package as a new chunk and updates the manifest. evrFileTools is built in, so no external tool runs.
5. **Install other mods** (in Settings) adds a mod made with other tools. Pick its `.zip` and its files are copied into the staging folder (`Settings/input-quest` or `Settings/input-pcvr`), so the next repack applies them together with your own changes. Zips with the type folders at the top, inside a wrapper folder, or inside evrFileTools' chunk folder all work; readmes and images in the zip are ignored. A mod built for the other platform is refused, and a mod's own cosmetic database is left out, since the editor writes the database on every repack.
6. **Revert** puts back the original manifest and deletes only the chunks the editor added. The manifest is backed up automatically, along with a note of how many chunks the install shipped with, before the first repack.

### Quest

The Android app runs on the headset and edits the game's files in place.

- **File access:** on first start the app asks for "All files access", which it needs to read and write Echo VR's files. Switch it on for EchoVR Cosmetics and return to the app. You can also grant it from a PC:
  `adb shell appops set com.echotools.cosmeticeditor MANAGE_EXTERNAL_STORAGE allow`
- **Data locations:** the app looks in both places Echo VR's data can live:
  - `/sdcard/Android/media/com.readyatdawn.r15/files/_data/<id>/rad15/android`
  - `/sdcard/readyatdawn/files/_data/<id>/rad15/android`

  If both exist, a repack mods both, so the change shows up whichever one the game loads. Previews come from the first.
- **Repacking:** a Quest install ships one package chunk (`48037dc70b0ecab2_0`), so a repack appends `_1`. A PC install ships `_0` to `_2`, so a repack appends `_3`.
- **Cosmetic database:** Quest ships the database under two names, `r14_glb_global_root` and `r14_glb_global_root_lowspec`. Which one loads depends on the headset, so both are written.
- **Extraction:** Quest mode only repacks. The full game data is too large to extract on the headset, and nothing needs it.

### Textures

Echo textures have an Echo header followed by the pixel data, either inline or in a separate GPU file. The editor reads the real header layout, whose fields start at `+0xC0`. PC headers are 0x100 bytes and wrap a DDS file; Quest headers are 0xF8 bytes followed by raw ASTC blocks.

- **PC:** textures are BCn. `ms_texconv.exe` in `Settings/` converts them to PNG for previews and converts replacement PNGs to DDS.
- **Quest:** textures are ASTC (4x4 and 8x8, sRGB or linear).
  - Decoding is done in pure Go.
  - Encoding uses astc-encoder, compiled into the app.
  - A replacement is rebuilt to match the original: same size, mip count, block size, colour space, and inline-or-GPU storage.
  - Mips are generated with correct gamma for sRGB textures and alpha-aware filtering.

PC and Quest store the same resource types under different folder hashes, because the type names carry a `Win10` or `Android` suffix before hashing. The app derives every hash from its type name; the tests pin the results against values checked in the game binaries.

## Requirements

- Echo VR game data (the `_data` directory)
- **Windows:** `ms_texconv.exe` in the `Settings/` folder (included; MIT licensed, see `Settings/texconv-LICENSE.txt`)
- **Model import (PC, optional):** Python and Blender on `PATH`
- **Quest:** install the APK (`adb install -r EchoVR_Cosmetics.apk`) and allow "All files access" when asked

## Building

Both builds need Go 1.25+ and cgo, which compiles the built-in ASTC encoder.

Windows (with a MinGW GCC such as TDM-GCC on `PATH`):

```bash
go build -ldflags "-s -w" -o EchoVR-Cosmetics-Editor.exe .
```

Quest APK (needs the Android SDK and NDK):

```powershell
./build_android.ps1 -Sdk <Android SDK> -Ndk <NDK>
```

`-Sdk` and `-Ndk` can be left out if `ANDROID_HOME` and `ANDROID_NDK_HOME` are set. The script packages the app against a patched copy of Fyne in `.build/`: `patches/fyne-v2.7.3/android.go` drops the controller hover and scroll events that otherwise make the UI scroll by itself on a Quest. `go.mod` and `go.sum` are restored when it finishes. The C++ runtime is linked statically, so the APK has no extra native libraries.

### Tests

```bash
go test ./...
```

Tests that need real game data are skipped unless you point them at it:

| Variable | Used for |
|---|---|
| `EVR_DATA_DIR`, `EVR_PLATFORM` | reading textures and the database straight from a package |
| `EVR_QUEST_INSTALL` | repacking into a *copy* of a Quest install, texture replacement, tint thumbnails |
| `EVR_EMBEDDED_DB` | comparing the shipped database with the embedded one |
| `EVR_QUEST_EXTRACT` | decoding every texture in a full Quest extract |

## Project Layout

| Path | What's there |
|---|---|
| `main.go`, `questsetup.go` | App start-up, main window, Quest file access and data detection |
| `Cosmetics/` | One editor per cosmetic category |
| `Data/` | App state, platform hashes and paths, package reading, repack/revert, texture caching and replacement |
| `Texture/` | Echo texture headers, format tables, ASTC and uncompressed decoding |
| `Texture/astcenc/` | astc-encoder 5.7.0 (Apache-2.0) and its cgo wrapper |
| `EvrFile/` | evrFileTools' package and manifest code, ported to run in process |
| `patches/` | The Fyne patch for Quest input |
| `Settings/` | `ms_texconv.exe`; the app also writes its caches and staging folders here |

## Credits

- [he_is_the_cat](https://github.com/heisthecat31): Cosmetic Editor author
- [goopsie](https://github.com/goopsie): evrFileTools, original cosmetic RE work, Texture Editor
- [astc-encoder](https://github.com/ARM-software/astc-encoder) by Arm (Apache-2.0), [DirectXTex texconv](https://github.com/microsoft/DirectXTex) by Microsoft (MIT), [Fyne](https://fyne.io) (BSD-3-Clause)
- Built for the Echo VR community
