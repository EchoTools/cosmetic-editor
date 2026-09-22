# Changelog

All notable changes to EchoVR Cosmetics Editor are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [8.1.0] - 2026-09-21

### Added
- **Quest app**: the editor runs on the Quest as an Android APK, edits cosmetics and repacks them into Echo VR on the headset. It asks for "All files access" on first launch and finds the game's data by itself.
- **Both Quest install locations**: Echo VR data under `Android/media/com.readyatdawn.r15` and `/sdcard/readyatdawn` is found, and a repack mods every install present.
- **Quest textures**: pure-Go ASTC decoding for previews, and astc-encoder built in for replacements. Replacements match the original's size, mips, block size, colour space and inline/GPU storage.
- **Install other mods** (Settings): adds a mod `.zip` made with other tools to the staged files, so the next repack applies it with the editor's own changes. Mods for the other platform are refused; a mod's own cosmetic database is skipped.
- **Tint colour picker** next to each hex field; hex input is validated as you type.

### Changed
- evrFileTools is built in and runs in process; repacking no longer needs extraction. Previews are read straight from the game's package.
- The cosmetic database is written under both Quest asset names (`r14_glb_global_root` and `_lowspec`).
- Revert removes exactly the chunks the editor added (`_1` onwards on Quest, `_3` onwards on PC), using a chunk count recorded with the manifest backup.
- Quest mode hides chassis, bracers, boosters and fanfare audio.
- Model import (Blender/GLB) for chassis, bracers and boosters is PC only. The game's base mesh is now read straight from the package instead of an extracted copy.
- The external `evrfiletools.exe`, `astcenc-avx2.exe` and helper DLLs have been removed; both tools are built in.

### Fixed
- Quest controller pointer no longer makes the UI scroll on its own or turns clicks into drags.
- Tint edits no longer reset when clicking away; edits are saved to disk immediately.
- Generate Thumbnail uses the tint's real colours instead of solid grey, and only appears on the Tints tab.
- Selecting an item no longer freezes the app while its texture loads; previews load in the background.
- Missing previews for banners, tags, emblems, decals, medals, pips and patterns.

## [1.1.1] - 2026-05-11

### Added
- **Preview with Tint**: Enabled the "Preview with Tint" feature for Banners, Tags, Emblems, Decals, and Pips.

### Fixed
- **Tint Previews**: Fixed a bug where the tint preview was not working on Patterns due to incorrect index mapping in `RefreshAssetPreview`.
- **UI Thread Safety**: Resolved multiple thread safety errors ("Error in Fyne call thread") by wrapping background UI updates in `fyne.Do`.
- **Medal Tints**: Ensured that tints are not applied to Medals even if the state was active when switching tabs.

## [1.1.0] - 2026-05-01

### Changed
- **Banners Editor**: Removed `Width` and `Height` text inputs for Emblems and Medals. These underlying values (`unk1` and `unk2`) are now recognized as internal scaling/shader properties rather than literal 2D texture dimensions, so hiding them prevents accidental corruption of banner decal mappings.

### Fixed
- **Tint Previews**: Fixed an issue where Tints were not rendering correctly on top of asset previews by resolving image boundaries and RGBA mutation loops in `ApplyTintToImage`.
- **Medal Tints**: Removed the erroneous "Preview with Tint" toggle option for Medals, as Medals natively do not support custom tinting.
- **Replacement Texture Persistence**: Resolved a critical bug in `banners.go` where `LoadToEditor` was failing to pass the `CurrentReplacementPath` state variable to the preview refresher, causing replacement textures to disappear visually when other UI elements were updated.

## [1.0.0] - 2024-01-01

### Added
- Initial public release of EchoVR Cosmetics Editor.
- GUI built with the Fyne toolkit for editing Echo VR cosmetic data files.
- **Tints** tab: RGB color editing for primary and secondary tint layers with live hex input.
- **Titles** tab: Edit title text, display names, rarities, and thumbnail symbols.
- **Emissives** tab: Multi-color gradient editing with live color preview and scrolling-texture support.
- **Fanfares** tab: Edit fanfare cosmetic metadata and rarity.
- **Emotes** tab: GIF-to-emote conversion with automatic DDS frame splitting; animated preview.
- **Banners** tab: High-resolution PNG texture replacement with medal/emblem position controls.
- **Tags** tab: Texture replacement for player tags.
- **Emblems** tab: Texture replacement for emblems.
- **Decals** tab: Texture replacement for decal cosmetics.
- **Medals** tab: Texture replacement for medals.
- **Pips** tab: Texture replacement for pip cosmetics.
- **Patterns** tab: Texture replacement for chassis patterns.
- **Metadata Editor**: Edit display names, internal names, descriptions, and rarity for all cosmetic types.
- **Rarity Control**: Full support for Default, Common, Fine, Superb, Epic, Legendary, and Mythic rarity levels.
- **Thumbnail Management**: Generate tinted thumbnails and assign thumbnail symbols.
- **Texture Cache**: Automatic extraction and caching of original game textures.
- **Quick Swap / Repack**: Integration with `evrFileTools` for injecting modified assets into game package files.
- **Revert**: One-click restore of the original manifest with cleanup of modifications.
- **Automatic Backup**: `.bak` manifest backup created before any modification.
- **PCVR / Quest mode**: Toggle between PC VR and Quest cosmetic data sets.
- **Settings dialog**: Configure paths for EchoVR data, extracted assets, and texture cache.
- Embedded default cosmetic database for first-run experience.
- Autosave to a temp file for crash recovery.
