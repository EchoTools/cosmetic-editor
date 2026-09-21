# Cosmetic Editor

A Go/Fyne GUI for editing Echo VR cosmetics — tints, emotes, emissives, banners, tags, medals, and metadata. Built by [he_is_the_cat](https://github.com/heisthecat31), based on [goopsie](https://github.com/goopsie)'s reverse engineering work and `evrFileTools`.

## What It Does

The Cosmetic Editor lets you modify everything about a cosmetic item: textures, colors, animations, display names, descriptions, and rarity. Changes are saved to a temp `.bin` file in memory until you explicitly write them — only "Save Data File" overwrites the game's cosmetic dictionary and updates the file header (e.g. when emote frame counts change).

### vs. Texture Editor

The [Texture Editor](https://github.com/EchoTools/EchoVR-Texture-Editor) (goopsie's Python tool) gives you access to all ~12,000 individual textures in the game data, but is limited to raw texture replacement — you can swap a tag's image, but can't touch its metadata, name, or rarity.

The Cosmetic Editor is narrower in scope (cosmetics only, not all textures) but deeper — full control over every cosmetic property. Both tools share the same folder layout (`Settings/texture_cache/`) and can be used side-by-side.

| | Texture Editor | Cosmetic Editor |
|---|---|---|
| Scope | All ~12k textures | Cosmetic items only |
| Language | Python (single script) | Go (structured, Fyne GUI) |
| Metadata editing | No | Yes (names, descriptions, rarity) |
| Tints | No | Yes (primary/secondary RGB) |
| Emissives | No | Yes (gradients, scroll presets) |
| Emotes | No | Yes (GIF import, frame slicing, export) |
| Runs on Quest | No | Yes (Android APK, edits and repacks on the headset) |
| Extraction/repacking | Yes (via evrFileTools) | Repacking built in (ported evrFileTools) |

### Cosmetic Categories

- **Tints** — Primary and secondary RGB color layers
- **Emotes** — GIF-to-emote conversion: import a GIF, it gets sliced into frames, converted to DDS, headers added, and repacked. Export existing emotes as GIF.
- **Emissives** — Scrolling texture presets and multi-color gradients
- **Banners, Tags, Emblems** — High-res texture replacement
- **Medals, Pips, Patterns, Titles, Decals, Fanfares** — Full metadata and texture editing
- **Metadata** — Display names, internal names, descriptions
- **Rarity** — Mythic, Legendary, Epic, Superb, Fine, Common, Default

Unused assets exist for bracers, chassis, and boosters — these are present in the data but can't be modified until model editing is possible.

## How It Works

1. Point the editor at your Echo VR `_data` directory
2. Previews are decoded straight from the game's package files and cached in `Settings/texture_cache/`
3. Edit cosmetics — changes are held in memory as temp `.bin` files
4. **Save Data File** — writes metadata changes (names, rarities, descriptions) to the game's cosmetic dictionary, updating the header if sizes changed
5. **Repack & Apply** — appends the modified assets to the game's package files (evrFileTools, built in). On a Quest with Echo VR in both `Android/media/com.readyatdawn.r15` and `/sdcard/readyatdawn`, both installs are modded
6. **Revert** — restores the original manifest and cleans up modifications (`.bak` files are created automatically before any write)

### File Conversion Pipeline

On PC, PNG to DDS conversion uses `ms_texconv.exe` (in `Settings/`). On Quest, textures are encoded to ASTC in process (astc-encoder, built in) at the original texture's size and mip count.

## Requirements

- Echo VR game data (`_data` directory)
- Windows: `ms_texconv.exe` in the `Settings/` folder (included)
- Quest: install the APK and allow "All files access" when asked

### Quest Status

PC texture modifications work fully. Quest cosmetic modifications (e.g. tags) currently show as corrupted in-game — the cause is under investigation. Basic texture swaps on Quest work fine.

## Building

Windows (needs cgo, e.g. TDM-GCC, for the built-in ASTC encoder):

```bash
go build -ldflags "-s -w" -o EchoVR-Cosmetics-Editor.exe .
```

Quest APK (needs the Android SDK and NDK):

```powershell
./build_android.ps1 -Sdk <Android SDK> -Ndk <NDK>
```

Requires Go 1.25+ and Fyne v2 dependencies.

## Credits

- [he_is_the_cat](https://github.com/heisthecat31) — Cosmetic Editor author
- [goopsie](https://github.com/goopsie) — evrFileTools, original cosmetic RE work, Texture Editor
- Built for the Echo VR community
