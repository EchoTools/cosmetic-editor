# EchoVR Cosmetics Editor

Echo VR cosmetic editor GUI written in go, based on [Goopsie](https://github.com/goopsie)'s EVR cosmetic RE work.

## 🚀 Core Features

### 🎨 Comprehensive Cosmetic Support
Modify every category of cosmetics available in EchoVR:
- **Tints**: Full RGB color editing for Primary and Secondary layers.
- **Emotes**: Advanced GIF-to-Emote conversion with 1:1 engine parity.
- **Emissives**: Custom scrolling texture presets and multi-color gradient support.
- **Banners & Tags**: High-resolution texture replacement for banners, tags, and emblems.
- **Metadata Editor**: Edit Display Names, Internal Names, and Descriptions.
- **Rarity Control**: Full support for all rarity levels (Mythic, Legendary, Epic, Superb, Fine, Common, Default).

### 📦 Advanced Repacking Engine
Built-in integration with `evrFileTools` for seamless asset application:
- **Quick Swap**: Fast, incremental repacking into the game's data packages.
- **Specialized Revert**: One-click restoration of the original manifest and modification cleanup.
- **Automatic Backup**: Creation of `.bak` manifest files before any modification.

### 🖼️ Visual Workflow
- **Live Previews**: See your Emotes animate and your Emissive gradients update in real-time.
- **Texture Cache**: Automatically extracts and caches original textures for previewing.
- **Thumbnail Management**: Generate and assign thumbnail symbols for your custom creations.

## 🛠️ Installation & Setup

### Pre-built release (recommended)

1. **Download**: Grab the latest `EchoVR-Cosmetics-Editor.exe` from the [Releases](../../releases) page.
2. **Tools**: Place `evrFileTools.exe` and `texconv.exe` in the `settings` folder next to the executable.
3. **Data Path**: On first launch use **Settings** (⚙) to set your EchoVR `_data` directory (e.g. `.../ready-at-dawn-echo-arena/_data/5932408047/rad15/win10`).
4. **Extract Assets**: Click **EXTRACT ASSETS** in the Action Bar to prepare the original textures for previewing.

### Prerequisites (build from source)

| Requirement | Version |
|-------------|---------|
| [Go](https://go.dev/dl/) | 1.25+ |
| [Fyne CLI](https://docs.fyne.io/started/) (`go install fyne.io/fyne/v2/cmd/fyne@latest`) | latest |
| GCC / MinGW (Windows) | any recent |

### Build from source

```bash
# Clone the repository
git clone https://github.com/EchoTools/EchoVR-Cosmetics-Editor.git
cd EchoVR-Cosmetics-Editor

# Download Go module dependencies
go mod download

# Quick development build (outputs a binary in the current directory)
go build -o EchoVR-Cosmetics-Editor.exe .

# Production package (embeds icon, sets Windows metadata — requires Fyne CLI)
fyne package -os windows -icon icon.ico -name "EchoVR-Cosmetics-Editor"
```

The `fyne package` command produces a self-contained `EchoVR-Cosmetics-Editor.exe` with the application icon and Windows metadata baked in.  Place `evrFileTools.exe` and `texconv.exe` in the `Settings/` folder alongside the built executable before running.

## 📖 Usage Guide

### Modifying Tints
1. Select the **Tints** tab.
2. Edit the Primary/Secondary hex codes.
3. Your changes are automatically saved to the database in memory.

### Creating Custom Emotes
1. Select the **Emotes** tab.
2. Click **REPLACE WITH GIF** and select your file.
3. The editor will automatically convert the GIF into split frames in DDS format.
4. Click **REPACK & APPLY** to see it in-game.

### Applying Changes
- **Save Data File**: Persists the metadata changes (Names, Rarities, etc.) to the game's cosmetic dictionary.
- **Repack & Apply**: Builds the modified binary assets (Textures, Emotes) and injects them into the game's package files.

## ⚠️ Important Notes
- **Backup!**: While the tool includes a specialized Revert system, it is always recommended to manually backup your `_data` folder before a major overhaul.
