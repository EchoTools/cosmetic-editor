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
| ADB push (Quest) | Yes | Not yet |
| Extraction/repacking | Yes (via evrFileTools) | Yes (via evrFileTools) |

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
2. Click **Extract Assets** to cache original textures to `Settings/texture_cache/` (shared with Texture Editor)
3. Edit cosmetics — changes are held in memory as temp `.bin` files
4. **Save Data File** — writes metadata changes (names, rarities, descriptions) to the game's cosmetic dictionary, updating the header if sizes changed
5. **Repack & Apply** — builds modified binary assets via `evrFileTools` and injects them into the game's package files
6. **Revert** — restores the original manifest and cleans up modifications (`.bak` files are created automatically before any write)

### File Conversion Pipeline

PNG to DDS conversion uses `texconv.exe` (in `Settings/`). The texture cache (~12k files) takes about 5 minutes to build on first run.

## Requirements

- `evrFileTools.exe` in the `Settings/` folder
- `texconv.exe` in the `Settings/` folder
- Echo VR game data (`_data` directory)
- Windows (native), Quest support is work-in-progress

### Quest Status

PC texture modifications work fully. Quest cosmetic modifications (e.g. tags) currently show as corrupted in-game — the cause is under investigation. Basic texture swaps on Quest work fine.

## Building

```bash
go build -o cosmetic-editor.exe .
```

Requires Go 1.22+ and Fyne v2 dependencies.

## Credits

- [he_is_the_cat](https://github.com/heisthecat31) — Cosmetic Editor author
- [goopsie](https://github.com/goopsie) — evrFileTools, original cosmetic RE work, Texture Editor
- Built for the Echo VR community
