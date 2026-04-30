# Changelog

All notable changes to EchoVR Cosmetics Editor are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

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
