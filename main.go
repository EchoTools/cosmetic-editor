package main

//go:generate rsrc -ico icon.ico

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// --- EMBEDDED FILES ---
//
//go:embed 0x43934c379cf1e366_original
var embeddedOriginal []byte

//go:embed icon.ico
var embeddedIcon []byte

//go:embed template_thumb.png
var embeddedTemplate []byte

// --- SETTINGS STRUCT ---
type AppSettings struct {
	AssetsPath      string `json:"assets_path"` // Now used for the auto-downloaded path
	TextureOutPath  string `json:"texture_out_path"`
	MetadataOutPath string `json:"metadata_out_path"`
	EchoVRDataPath  string `json:"echovr_data_path"`
	BackupPath      string `json:"backup_path"`
	Mode            string `json:"mode"`
}

// Global State
var (
	currentCList            cosmeticList
	tintIndices             []int
	filteredIndices         []int
	titleIndices            []int
	filteredTitleIndices    []int
	emissiveIndices         []int
	filteredEmissiveIndices []int
	fanfareIndices          []int
	filteredFanfareIndices  []int
	selectedListIndex       int = -1

	// Flags
	isLoadingEntry bool

	// UI
	titleStringEntry    *widget.Entry
	emissiveUnk1Entry   *widget.Entry
	emissiveUnk2Entry   *widget.Entry
	emissiveTexEntry    *widget.Entry
	emissiveColorsEntry *widget.Entry
	raritySelect        *widget.Select
	fanfare1Entry       *widget.Entry
	fanfare2Entry       *widget.Entry
	searchEntryFanfares *widget.Entry
	fanfareList         *widget.List

	// File Paths
	// tempFilePath is initialized in main() to ensure it uses the correct separator/path
	tempFilePath string
	settingsFile = "settings.json"
	appSettings  AppSettings
)

// --- RARITY DEFINITIONS ---
var rarityNameToSymbol = map[string]int64{
	"Default":   RarityDefault,
	"Common":    RarityCommon,
	"Fine":      RarityFine,
	"Superb":    RaritySuperb,
	"Epic":      RarityEpic,
	"Legendary": RarityLegendary,
	"Mythic":    RarityMythic,
}

var raritySymbolToName = map[int64]string{
	RarityDefault:   "Default",
	RarityCommon:    "Common",
	RarityFine:      "Fine",
	RaritySuperb:    "Superb",
	RarityEpic:      "Epic",
	RarityLegendary: "Legendary",
	RarityMythic:    "Mythic",
}

var rarityOptions = []string{"Default", "Common", "Fine", "Superb", "Epic", "Legendary", "Mythic"}

// --- CONSTANTS FOR PATHS ---
const (
	DefaultEchoPath = "./ready-at-dawn-echo-arena/_data/5932408047/rad15/win10"
	PackageName     = "48037dc70b0ecab2"

	// Base Directories
	SettingsDirName = "Settings"
	ExtractedDir    = "Settings/pcvr-extracted"
	OutputDir       = "Settings/output-both"
	BackupDirName   = "Backup" // Folder name inside Settings

	// PCVR Constants
	InputDirNamePC    = "input-pcvr"
	TintFolderPC      = "0/32f30fe361939dee"
	TintFileNamePC    = "43934c379cf1e366"
	ThumbTexFolderPC  = "0/beac1969cb7b8861"
	ThumbMetaFolderPC = "0/4a4c32c49300b8a0"

	// Quest Constants
	InputDirNameQuest    = "input-quest"
	TintFolderQuest      = "0/24cbfd54e9a7f2ea"
	TintFileNameQuest    = "bb758e9809650e3b"
	ThumbTexFolderQuest  = "0/489bb35d53ca50e9"
	ThumbMetaFolderQuest = "0/e2ef0854d0cd69b8"
)

// Dynamic Paths (Initialized to PCVR defaults)
var (
	InputDirName    = InputDirNamePC
	TintFolder      = TintFolderPC
	TintFileName    = TintFileNamePC
	ThumbTexFolder  = ThumbTexFolderPC
	ThumbMetaFolder = ThumbMetaFolderPC
)

// --- HELPER: Find Tool Path ---
// Looks for tools in ./Settings OR [ExeDir]/Settings
func findTool(toolName string) (string, error) {
	// 1. Check Working Directory (./Settings/tool.exe)
	localPath := filepath.Join(SettingsDirName, toolName)
	if _, err := os.Stat(localPath); err == nil {
		path, _ := filepath.Abs(localPath)
		return path, nil
	}

	// 2. Check Executable Directory ([ExeDir]/Settings/tool.exe)
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		exeToolPath := filepath.Join(exeDir, SettingsDirName, toolName)
		if _, err := os.Stat(exeToolPath); err == nil {
			return exeToolPath, nil
		}
	}

	return "", fmt.Errorf("tool '%s' not found in %s folder", toolName, SettingsDirName)
}

// --- HELPER: Get Settings Dir Path ---
// Returns the best guess for the absolute path to the Settings folder
func getSettingsDir() string {
	// Try local first
	if _, err := os.Stat(SettingsDirName); err == nil {
		path, _ := filepath.Abs(SettingsDirName)
		return path
	}
	// Try next to executable
	exePath, err := os.Executable()
	if err == nil {
		return filepath.Join(filepath.Dir(exePath), SettingsDirName)
	}
	return SettingsDirName // Fallback to relative
}

func updateMode(mode string) {
	if mode == "Quest" {
		TintFolder = TintFolderQuest
		TintFileName = TintFileNameQuest
		ThumbTexFolder = ThumbTexFolderQuest
		ThumbMetaFolder = ThumbMetaFolderQuest
		InputDirName = InputDirNameQuest
	} else {
		TintFolder = TintFolderPC
		TintFileName = TintFileNamePC
		ThumbTexFolder = ThumbTexFolderPC
		ThumbMetaFolder = ThumbMetaFolderPC
		InputDirName = InputDirNamePC
	}
	appSettings.Mode = mode
}

func main() {
	os.Setenv("FYNE_GL_VERSION", "2.1")

	// 1. SETUP: Directories & Settings
	settingsPath := getSettingsDir()
	os.MkdirAll(settingsPath, 0755)

	// Create Settings/Temp folder
	tempDir := filepath.Join(settingsPath, "Temp")
	os.MkdirAll(tempDir, 0755)

	// Set temp file path
	tempFilePath = filepath.Join(tempDir, "temp_autosave.dat")

	loadSettings()
	if appSettings.EchoVRDataPath == "" {
		appSettings.EchoVRDataPath = DefaultEchoPath
	}
	// Default assets path to Settings/thumbnail
	if appSettings.AssetsPath == "" || appSettings.AssetsPath == "./thumbnails" {
		appSettings.AssetsPath = filepath.Join(settingsPath, "thumbnail")
	}
	if appSettings.Mode == "" {
		appSettings.Mode = "PCVR"
	}
	updateMode(appSettings.Mode)

	saveSettings()

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("CRITICAL ERROR:", r)
			time.Sleep(10 * time.Second)
		}
	}()

	// --- DOWNLOAD THUMBNAILS ON START ---
	go downloadAndExtractThumbnails()

	a := app.New()
	a.SetIcon(fyne.NewStaticResource("icon.ico", embeddedIcon))

	w := a.NewWindow("EchoVR Cosmetics Editor")
	w.Resize(fyne.NewSize(1100, 800))

	// --- UI DEFINITIONS ---
	statusLabel := widget.NewLabel("Status: Initializing...")

	nameEntry := widget.NewEntry()
	descEntry := widget.NewEntry()
	thumbIdEntry := widget.NewEntry()
	titleStringEntry = widget.NewEntry()
	emissiveUnk1Entry = widget.NewEntry()
	emissiveUnk2Entry = widget.NewEntry()
	emissiveTexEntry = widget.NewEntry()
	emissiveColorsEntry = widget.NewMultiLineEntry()
	raritySelect = widget.NewSelect(rarityOptions, nil)
	fanfare1Entry = widget.NewEntry()
	fanfare2Entry = widget.NewEntry()
	searchEntryFanfares = widget.NewEntry()
	searchEntryFanfares.PlaceHolder = "Search Fanfares..."
	fanfareList = widget.NewList(
		func() int { return len(filteredFanfareIndices) },
		func() fyne.CanvasObject { return widget.NewLabel("Template") },
		func(id widget.ListItemID, item fyne.CanvasObject) {
			realIndex := filteredFanfareIndices[id]
			entry := currentCList.cosmeticEntries[realIndex]
			dName := string(bytes.TrimRight(entry.cEntry.DisplayNameString[:], "\x00"))
			item.(*widget.Label).SetText(dName)
		},
	)

	thumbImage := canvas.NewImageFromResource(nil)
	thumbImage.FillMode = canvas.ImageFillContain
	thumbImage.SetMinSize(fyne.NewSize(150, 150))
	thumbContainer := container.NewMax(
		canvas.NewRectangle(color.RGBA{30, 30, 30, 255}),
		thumbImage,
	)

	// Emissive Preview UI
	emissivePreviewImage := canvas.NewImageFromImage(nil)
	emissivePreviewImage.FillMode = canvas.ImageFillContain
	emissivePreviewImage.SetMinSize(fyne.NewSize(150, 150))
	emissivePreviewContainer := container.NewMax(
		canvas.NewRectangle(color.RGBA{30, 30, 30, 255}),
		emissivePreviewImage,
	)
	emissivePreviewLabel := widget.NewLabel("Color Preview")
	emissivePreviewWrapper := container.NewCenter(emissivePreviewContainer)
	emissivePreviewLabel.Hide()
	emissivePreviewWrapper.Hide()

	// HEX ENTRIES (Main/Secondary)
	primaryHex := widget.NewEntry()
	primaryHex.PlaceHolder = "FFFFFF"
	secondaryHex := widget.NewEntry()
	secondaryHex.PlaceHolder = "FFFFFF"

	// Validators
	thumbIdEntry.Validator = func(s string) error {
		_, err := strconv.ParseInt(s, 10, 64)
		return err
	}
	hexValidator := func(s string) error {
		s = strings.TrimPrefix(s, "#")
		if len(s) != 6 {
			return fmt.Errorf("must be 6 chars")
		}
		if _, err := hex.DecodeString(s); err != nil {
			return fmt.Errorf("invalid hex")
		}
		return nil
	}
	primaryHex.Validator = hexValidator
	secondaryHex.Validator = hexValidator

	// --- FILE OPERATIONS ---
	saveToTemp := func() {
		b, err := cosmeticListToBytes(currentCList)
		if err != nil {
			fmt.Println("Error saving temp:", err)
			return
		}
		os.WriteFile(tempFilePath, b, 0644)
	}

	loadFromTemp := func() {
		if _, err := os.Stat(tempFilePath); os.IsNotExist(err) {
			dataCopy := make([]byte, len(embeddedOriginal))
			copy(dataCopy, embeddedOriginal)
			os.WriteFile(tempFilePath, dataCopy, 0644)
		}
		b, err := os.ReadFile(tempFilePath)
		if err != nil {
			statusLabel.SetText("Error reading temp file!")
			return
		}
		cList, err := bytesToCosmeticList(b)
		if err != nil {
			statusLabel.SetText("Error parsing temp file!")
			return
		}
		currentCList = cList
	}

	// --- LOGIC: APPLY CHANGES ---
	var tabs *container.AppTabs
	applyChange := func(modifier func(interface{})) {
		if isLoadingEntry {
			return
		}
		if selectedListIndex == -1 {
			return
		}

		idx := selectedListIndex
		entry := currentCList.cosmeticEntries[idx]
		var newEntry cosmeticEntry
		var err error

		if tabs.Selected().Text == "Tints" {
			t := &CTint{}
			if err := t.FromCosmeticEntry(entry); err != nil {
				return
			}
			modifier(t)
			newEntry, err = t.ToCosmeticEntry()
		} else if tabs.Selected().Text == "Titles" {
			t := &CTitle{}
			if err := t.FromCosmeticEntry(entry); err != nil {
				return
			}
			modifier(t)
			newEntry, err = t.ToCosmeticEntry()
		} else if tabs.Selected().Text == "Emissives" {
			t := &CEmissive{}
			if err := t.FromCosmeticEntry(entry); err != nil {
				return
			}
			modifier(t)
			newEntry, err = t.ToCosmeticEntry()
		} else if tabs.Selected().Text == "Fanfares" {
			t := &CFanfare{}
			if err := t.FromCosmeticEntry(entry); err != nil {
				return
			}
			modifier(t)
			newEntry, err = t.ToCosmeticEntry()
		}

		if err != nil {
			return
		}

		// Fix Size Difference
		oldSize := len(currentCList.cosmeticEntries[idx].cEntryExtData)
		newSize := len(newEntry.cEntryExtData)
		currentCList.listSize = uint64(int64(currentCList.listSize) + int64(newSize-oldSize))
		currentCList.cosmeticEntries[idx] = newEntry

		saveToTemp()
	}

	// --- LISTENERS ---
	onName := func(s string) {
		applyChange(func(v interface{}) {
			switch t := v.(type) {
			case *CTint:
				t.DisplayName = s
			case *CTitle:
				t.DisplayName = s
			case *CEmissive:
				t.DisplayName = s
			case *CFanfare:
				t.DisplayName = s
			}
		})
	}
	onDesc := func(s string) {
		applyChange(func(v interface{}) {
			switch t := v.(type) {
			case *CTint:
				t.Description = s
			case *CTitle:
				t.Description = s
			case *CEmissive:
				t.Description = s
			case *CFanfare:
				t.Description = s
			}
		})
	}

	refreshThumbnail := func(idStr string) {
		if idStr == "" {
			return
		}
		// Check Auto-Downloaded Assets Path
		if appSettings.AssetsPath != "" {
			// Look for png files in the folder (e.g. symbol.png)
			// The zip usually contains images named by symbol
			originalPath := filepath.Join(appSettings.AssetsPath, idStr+".png")
			if _, err := os.Stat(originalPath); err == nil {
				thumbImage.File = originalPath
				thumbImage.Refresh()
				return
			}
		}

		// Fallback: Check Settings/thumbnail explicitly
		fallbackPath := filepath.Join(settingsPath, "thumbnail", idStr+".png")
		if _, err := os.Stat(fallbackPath); err == nil {
			thumbImage.File = fallbackPath
			thumbImage.Refresh()
			return
		}

		thumbImage.File = ""
		thumbImage.Refresh()
	}

	onThumb := func(s string) {
		refreshThumbnail(s)
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			applyChange(func(v interface{}) {
				switch t := v.(type) {
				case *CTint:
					t.ThumbnailSymbol = id
				case *CTitle:
					t.ThumbnailSymbol = id
				case *CEmissive:
					t.ThumbnailSymbol = id
				case *CFanfare:
					t.ThumbnailSymbol = id
				}
			})
		}
	}

	// Helper to handle hex input
	onHexChange := func(isPrimary bool) func(string) {
		return func(s string) {
			if tabs.Selected().Text != "Tints" {
				return
			}
			s = strings.TrimPrefix(s, "#")
			if len(s) != 6 {
				return
			}
			bytes, err := hex.DecodeString(s)
			if err != nil {
				return
			}
			r, g, b := float32(bytes[0])/255.0, float32(bytes[1])/255.0, float32(bytes[2])/255.0

			applyChange(func(v interface{}) {
				if t, ok := v.(*CTint); ok {
					if isPrimary {
						t.SecondaryColor_R, t.SecondaryColor_G, t.SecondaryColor_B = r, g, b
					} else {
						t.PrimaryColor_R, t.PrimaryColor_G, t.PrimaryColor_B = r, g, b
					}
				}
			})
		}
	}

	onRarityChange := func(s string) {
		selectedRaritySymbol := rarityNameToSymbol[s]
		applyChange(func(v interface{}) {
			switch t := v.(type) {
			case *CTint:
				t.Rarity = selectedRaritySymbol
			case *CTitle:
				t.Rarity = selectedRaritySymbol
			case *CEmissive:
				t.Rarity = selectedRaritySymbol
			case *CFanfare:
				t.Rarity = selectedRaritySymbol
			}
		})
	}

	onFanfare1Change := func(s string) {
		if id, err := strconv.ParseUint(s, 10, 32); err == nil {
			applyChange(func(v interface{}) {
				if t, ok := v.(*CFanfare); ok {
					t.Fanfare1ID = uint32(id)
				}
			})
		}
	}
	onFanfare2Change := func(s string) {
		if id, err := strconv.ParseUint(s, 10, 32); err == nil {
			applyChange(func(v interface{}) {
				if t, ok := v.(*CFanfare); ok {
					t.Fanfare2ID = uint32(id)
				}
			})
		}
	}

	titleStringEntry.OnChanged = func(s string) {
		applyChange(func(v interface{}) {
			if t, ok := v.(*CTitle); ok {
				t.TitleString = s
			}
		})
	}

	// Emissive Listeners
	emissiveUnk1Entry.OnChanged = func(s string) {
		if f, err := strconv.ParseFloat(s, 32); err == nil {
			applyChange(func(v interface{}) {
				if t, ok := v.(*CEmissive); ok {
					t.Unk1 = float32(f)
				}
			})
		}
	}
	emissiveUnk2Entry.OnChanged = func(s string) {
		if f, err := strconv.ParseFloat(s, 32); err == nil {
			applyChange(func(v interface{}) {
				if t, ok := v.(*CEmissive); ok {
					t.Unk2 = float32(f)
				}
			})
		}
	}
	emissiveTexEntry.OnChanged = func(s string) {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			applyChange(func(v interface{}) {
				if t, ok := v.(*CEmissive); ok {
					t.TextureSymbol = id
				}
			})
		}
	}

	refreshEmissivePreview := func(colors [][3]float32) {
		if len(colors) == 0 {
			emissivePreviewImage.Image = nil
			emissivePreviewImage.Refresh()
			return
		}
		width := 100
		height := 100
		img := image.NewRGBA(image.Rect(0, 0, width, height))

		sectionHeight := float64(height) / float64(len(colors))

		for i, c := range colors {
			col := color.RGBA{
				R: uint8(c[0] * 255),
				G: uint8(c[1] * 255),
				B: uint8(c[2] * 255),
				A: 255,
			}
			startY := int(float64(i) * sectionHeight)
			endY := int(float64(i+1) * sectionHeight)
			if i == len(colors)-1 {
				endY = height
			}

			for y := startY; y < endY; y++ {
				for x := 0; x < width; x++ {
					img.Set(x, y, col)
				}
			}
		}
		emissivePreviewImage.Image = img
		emissivePreviewImage.Refresh()
	}

	onEmissiveColorsChanged := func(s string) {
		lines := strings.Split(s, "\n")
		var newColors [][3]float32
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			line = strings.TrimPrefix(line, "#")
			if len(line) != 6 {
				continue
			}
			b, err := hex.DecodeString(line)
			if err != nil {
				continue
			}
			newColors = append(newColors, [3]float32{
				float32(b[0]) / 255.0,
				float32(b[1]) / 255.0,
				float32(b[2]) / 255.0,
			})
		}
		refreshEmissivePreview(newColors)
		// Only update if we parsed something valid to avoid clearing on typing
		if len(newColors) > 0 {
			applyChange(func(v interface{}) {
				if t, ok := v.(*CEmissive); ok {
					t.Colors = newColors
				}
			})
		}
	}
	emissiveColorsEntry.OnChanged = onEmissiveColorsChanged

	unbindListeners := func() {
		nameEntry.OnChanged = nil
		descEntry.OnChanged = nil
		thumbIdEntry.OnChanged = nil
		primaryHex.OnChanged = nil
		secondaryHex.OnChanged = nil
		titleStringEntry.OnChanged = nil
		emissiveUnk1Entry.OnChanged = nil
		emissiveUnk2Entry.OnChanged = nil
		emissiveTexEntry.OnChanged = nil
		emissiveColorsEntry.OnChanged = nil
		raritySelect.OnChanged = nil
		fanfare1Entry.OnChanged = nil
		fanfare2Entry.OnChanged = nil
	}

	bindListeners := func() {
		nameEntry.OnChanged = onName
		descEntry.OnChanged = onDesc
		thumbIdEntry.OnChanged = onThumb
		raritySelect.OnChanged = onRarityChange
		fanfare1Entry.OnChanged = onFanfare1Change
		fanfare2Entry.OnChanged = onFanfare2Change

		if tabs.Selected().Text == "Tints" {
			primaryHex.OnChanged = onHexChange(true)
			secondaryHex.OnChanged = onHexChange(false)
		} else if tabs.Selected().Text == "Titles" {
			titleStringEntry.OnChanged = func(s string) {
				applyChange(func(v interface{}) {
					if t, ok := v.(*CTitle); ok {
						t.TitleString = s
					}
				})
			}
		} else if tabs.Selected().Text == "Emissives" {
			emissiveUnk1Entry.OnChanged = func(s string) {
				if f, err := strconv.ParseFloat(s, 32); err == nil {
					applyChange(func(v interface{}) {
						if t, ok := v.(*CEmissive); ok {
							t.Unk1 = float32(f)
						}
					})
				}
			}
			emissiveUnk2Entry.OnChanged = func(s string) {
				if f, err := strconv.ParseFloat(s, 32); err == nil {
					applyChange(func(v interface{}) {
						if t, ok := v.(*CEmissive); ok {
							t.Unk2 = float32(f)
						}
					})
				}
			}
			emissiveTexEntry.OnChanged = func(s string) {
				if id, err := strconv.ParseInt(s, 10, 64); err == nil {
					applyChange(func(v interface{}) {
						if t, ok := v.(*CEmissive); ok {
							t.TextureSymbol = id
						}
					})
				}
			}
			emissiveColorsEntry.OnChanged = onEmissiveColorsChanged
		}
	}

	// --- THUMBNAIL GENERATOR ---
	generateAndSaveThumbnail := func() {
		// 1. Validate ID
		idStr := thumbIdEntry.Text
		if idStr == "" {
			dialog.ShowInformation("Error", "No Thumbnail ID (Symbol) set.", w)
			return
		}

		// Convert ID to Hex if it's decimal
		if val, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			idStr = fmt.Sprintf("%016x", uint64(val))
		}

		// 2. Load Embedded Template
		img, _, err := image.Decode(bytes.NewReader(embeddedTemplate))
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to decode embedded template: %v", err), w)
			return
		}

		// 3. Colors Helper
		parseColor := func(hexStr string) color.RGBA {
			hexStr = strings.TrimPrefix(hexStr, "#")
			b, _ := hex.DecodeString(hexStr)
			if len(b) < 3 {
				return color.RGBA{255, 255, 255, 255}
			}
			return color.RGBA{b[0], b[1], b[2], 255}
		}
		cPrim := parseColor(primaryHex.Text)
		cSec := parseColor(secondaryHex.Text)

		// Original Colors in Template to Replace
		srcPrimary := color.RGBA{0x9F, 0x12, 0x13, 0xFF}
		srcSecondary := color.RGBA{0xEC, 0xDB, 0x10, 0xFF}

		isSimilar := func(c1, c2 color.RGBA, threshold float64) bool {
			rDiff := float64(c1.R) - float64(c2.R)
			gDiff := float64(c1.G) - float64(c2.G)
			bDiff := float64(c1.B) - float64(c2.B)
			return math.Sqrt(rDiff*rDiff+gDiff*gDiff+bDiff*bDiff) < threshold
		}

		// 4. Create Tinted Image
		bounds := img.Bounds()
		dst := image.NewRGBA(bounds)

		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				srcC := img.At(x, y)
				r, g, b, a := srcC.RGBA()
				// Convert back to 8-bit
				currColor := color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}

				finalColor := currColor
				wasReplaced := false
				if isSimilar(currColor, srcPrimary, 20.0) {
					finalColor = color.RGBA{cSec.R, cSec.G, cSec.B, currColor.A}
					finalColor = color.RGBA{cPrim.R, cPrim.G, cPrim.B, currColor.A}
					wasReplaced = true
				} else if isSimilar(currColor, srcSecondary, 20.0) {
					finalColor = color.RGBA{cPrim.R, cPrim.G, cPrim.B, currColor.A}
					finalColor = color.RGBA{cSec.R, cSec.G, cSec.B, currColor.A}
					wasReplaced = true
				}

				if wasReplaced {
					darkenFactor := float32(0.6)
					r_dark := float32(finalColor.R) * darkenFactor
					g_dark := float32(finalColor.G) * darkenFactor
					b_dark := float32(finalColor.B) * darkenFactor
					finalColor.R = uint8(r_dark)
					finalColor.G = uint8(g_dark)
					finalColor.B = uint8(b_dark)
				}

				// Standard Orientation (No Flip)
				dst.Set(x, y, finalColor)
			}
		}

		// 5. Save to Temp PNG (for texconv)
		settingsPath := getSettingsDir()
		tempDir := filepath.Join(settingsPath, "Temp")
		os.MkdirAll(tempDir, 0755)

		tempPngPath := filepath.Join(tempDir, "temp_thumb.png")
		fPng, err := os.Create(tempPngPath)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		png.Encode(fPng, dst)
		fPng.Close()

		// 6. Find & Run Tool (Branch based on Mode)
		var generatedFile string

		if appSettings.Mode == "Quest" {
			// QUEST: Use astcenc-avx2.exe
			astcPath, err := findTool("astcenc-avx2.exe")
			if err != nil {
				dialog.ShowError(fmt.Errorf("astcenc-avx2.exe not found.\nPlease place it in:\n%s\n\nError: %v", filepath.Join(settingsPath, "astcenc-avx2.exe"), err), w)
				return
			}

			tempAstcPath := filepath.Join(tempDir, "temp_thumb.astc")
			// Command: astcenc-avx2.exe -cs [PngPath] [AstcPath] 6x6 -medium
			cmd := exec.Command(astcPath, "-cs", tempPngPath, tempAstcPath, "6x6", "-medium")

			if out, err := cmd.CombinedOutput(); err != nil {
				dialog.ShowError(fmt.Errorf("astcenc failed:\n%s", out), w)
				return
			}

			// Read generated ASTC file
			astcData, err := os.ReadFile(tempAstcPath)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to read generated astc: %v", err), w)
				return
			}

			// Validate Standard ASTC Header (Magic: 13 AB A1 5C)
			if len(astcData) < 16 || astcData[0] != 0x13 || astcData[1] != 0xAB || astcData[2] != 0xA1 || astcData[3] != 0x5C {
				dialog.ShowError(fmt.Errorf("generated file is not valid ASTC (wrong magic bytes)"), w)
				return
			}
			rawData := astcData[16:]

			// Construct Custom Quest Header (16 bytes)
			header := make([]byte, 16)
			header[0] = 0x53 // Magic
			header[1] = 0x80
			header[2] = 0x09 // Version
			header[3] = 0x00
			binary.LittleEndian.PutUint16(header[4:], 6)                    // Block Width (6)
			binary.LittleEndian.PutUint16(header[6:], 6)                    // Block Height (6)
			binary.LittleEndian.PutUint16(header[8:], uint16(bounds.Dx()))  // Image Width
			binary.LittleEndian.PutUint16(header[10:], uint16(bounds.Dy())) // Image Height
			// Bytes 12-15 are padding (0)

			finalData := append(header, rawData...)

			generatedFile = filepath.Join(tempDir, "temp_thumb_stripped.bin")
			if err := os.WriteFile(generatedFile, finalData, 0644); err != nil {
				dialog.ShowError(fmt.Errorf("failed to write stripped astc: %v", err), w)
				return
			}
			os.Remove(tempAstcPath)
		} else {
			// PCVR: Use texconv.exe
			texconvPath, err := findTool("texconv.exe")
			if err != nil {
				dialog.ShowError(fmt.Errorf("texconv.exe not found.\nPlease place it in:\n%s\n\nError: %v", filepath.Join(settingsPath, "texconv.exe"), err), w)
				return
			}
			// Command: texconv.exe -f BC7_UNORM_SRGB -y -o [TempDir] [PngPath]
			cmd := exec.Command(texconvPath, "-f", "BC7_UNORM_SRGB", "-y", "-o", tempDir, tempPngPath)
			if out, err := cmd.CombinedOutput(); err != nil {
				dialog.ShowError(fmt.Errorf("texconv failed:\n%s", out), w)
				return
			}
			generatedFile = filepath.Join(tempDir, "temp_thumb.dds")
		}

		// 7. Move DDS to Final Destination
		// Path: Settings/input-pcvr/0/-4707359568332879775/[ID]
		// Note: InputDir is relative, lets resolve it absolutely
		absInputDir := filepath.Join(settingsPath, InputDirName)
		texDir := filepath.Join(absInputDir, ThumbTexFolder)
		if err := os.MkdirAll(texDir, 0755); err != nil {
			dialog.ShowError(fmt.Errorf("failed to create tex dir: %v", err), w)
			return
		}

		outDdsPath := filepath.Join(texDir, idStr) // Saves as filename [ID] with no extension

		os.Remove(outDdsPath)
		if err := os.Rename(generatedFile, outDdsPath); err != nil {
			input, _ := os.ReadFile(generatedFile)
			os.WriteFile(outDdsPath, input, 0644)
		}

		// 8. Save Metadata File
		// Path: Settings/input-pcvr/0/5353709876897953952/[ID]
		metaDir := filepath.Join(absInputDir, ThumbMetaFolder)
		if err := os.MkdirAll(metaDir, 0755); err != nil {
			dialog.ShowError(fmt.Errorf("failed to create meta dir: %v", err), w)
			return
		}

		outMetaPath := filepath.Join(metaDir, idStr)
		if err := writeMetadata(outMetaPath); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save metadata: %v", err), w)
			return
		}

		// Cleanup
		os.Remove(tempPngPath)
		os.Remove(generatedFile)

		dialog.ShowInformation("Success", "Thumbnail generated.\nID: "+idStr, w)
		statusLabel.SetText("Generated: " + idStr)
	}

	btnGenThumb := widget.NewButton("Generate & Save Thumbnail", generateAndSaveThumbnail)

	// --- REPACK / EXTRACT LOGIC ---
	var showRepackDialog func() // Forward declaration

	// Recursive Copy Helper
	copyRecursive := func(src, dst string) error {
		return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			relPath, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			dstPath := filepath.Join(dst, relPath)

			if info.IsDir() {
				return os.MkdirAll(dstPath, info.Mode())
			}
			if !info.Mode().IsRegular() {
				return nil
			}

			sourceFile, err := os.Open(path)
			if err != nil {
				return err
			}
			defer sourceFile.Close()

			destinationFile, err := os.Create(dstPath)
			if err != nil {
				return err
			}
			defer destinationFile.Close()

			_, err = io.Copy(destinationFile, sourceFile)
			return err
		})
	}

	runExtract := func(echoPath string) error {
		// Points to Settings/evrFileTools.exe
		toolPath, err := findTool("evrFileTools.exe")
		if err != nil {
			return err
		}

		// Output Dir
		settingsPath := getSettingsDir()
		extractDir := filepath.Join(settingsPath, "pcvr-extracted")
		os.MkdirAll(extractDir, 0755)

		cmd := exec.Command(toolPath,
			"-mode", "extract",
			"-package", PackageName,
			"-data", echoPath,
			"-output", extractDir,
			"-export", "tints",
			"-force",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("extract failed: %v\nOutput: %s", err, string(out))
		}
		return nil
	}

	// Helper function for Part 1 of Repack: Execute Tool
	executeRepackTool := func(echoPath string) (string, error) {
		// 1. Prepare Paths
		settingsPath := getSettingsDir()
		absInputDir := filepath.Join(settingsPath, InputDirName)
		absOutputDir := filepath.Join(settingsPath, "output-both")

		// 2. Prepare Input File for Tints
		tintDir := filepath.Join(absInputDir, TintFolder)
		if err := os.MkdirAll(tintDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create tint dir: %v", err)
		}

		outFile := filepath.Join(tintDir, TintFileName)
		b, err := cosmeticListToBytes(currentCList)
		if err != nil {
			return "", fmt.Errorf("failed to serialize tint data: %v", err)
		}
		if err := os.WriteFile(outFile, b, 0644); err != nil {
			return "", fmt.Errorf("failed to write tint file: %v", err)
		}

		// 3. Run evrFileTools Replace
		os.MkdirAll(absOutputDir, 0755)

		toolPath, err := findTool("evrFileTools.exe")
		if err != nil {
			return "", err
		}

		cmd := exec.Command(toolPath,
			"-mode", "build",
			"-package", PackageName,
			"-data", echoPath,
			"-input", absInputDir,
			"-output", absOutputDir,
			"-force",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out), fmt.Errorf("repack failed: %v\nOutput: %s", err, string(out))
		}
		return string(out), nil
	}

	// Helper function for Part 2 of Repack: Push Files
	pushRepackedFiles := func(echoPath string) error {
		settingsPath := getSettingsDir()
		absOutputDir := filepath.Join(settingsPath, "output-both")

		srcPkg := filepath.Join(absOutputDir, "packages")
		dstPkg := filepath.Join(echoPath, "packages")
		if _, err := os.Stat(srcPkg); err == nil {
			if err := copyRecursive(srcPkg, dstPkg); err != nil {
				return fmt.Errorf("failed to move packages: %v", err)
			}
		}

		srcMan := filepath.Join(absOutputDir, "manifests")
		dstMan := filepath.Join(echoPath, "manifests")
		if _, err := os.Stat(srcMan); err == nil {
			if err := copyRecursive(srcMan, dstMan); err != nil {
				return fmt.Errorf("failed to move manifests: %v", err)
			}
		}

		return nil
	}

	showRepackDialog = func() {
		// Container for the modal content
		content := container.NewVBox()
		modal := dialog.NewCustom("Repack Tool", "Close", content, w)
		modal.Resize(fyne.NewSize(600, 400))

		// Check if extracted exists
		settingsPath := getSettingsDir()
		extractDir := filepath.Join(settingsPath, "pcvr-extracted")
		_, errExtract := os.Stat(extractDir)
		extractedExists := errExtract == nil

		// UI Elements
		lblPath := widget.NewLabel(appSettings.EchoVRDataPath)
		btnBrowse := widget.NewButton("Browse Data Path", func() {
			dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
				if uri != nil {
					appSettings.EchoVRDataPath = uri.Path()
					lblPath.SetText(appSettings.EchoVRDataPath)
					saveSettings()
				}
			}, w)
		})

		refreshUI := func() {
			content.Objects = nil // Clear

			if !extractedExists {
				// STATE 1: EXTRACT
				content.Add(widget.NewLabel("Step 1: Extract Original Tints"))
				content.Add(widget.NewLabel("Selected EchoVR Data Path:"))
				content.Add(container.NewBorder(nil, nil, nil, btnBrowse, lblPath))
				content.Add(widget.NewButton("Extract", func() {
					// Use a blocking dialog that runs extract in background to avoid freeze
					loading := dialog.NewCustom("Extracting...", "Cancel", widget.NewProgressBarInfinite(), w)
					loading.Show()

					go func() {
						err := runExtract(appSettings.EchoVRDataPath)
						loading.Hide()
						if err != nil {
							dialog.ShowError(err, w)
						} else {
							extractedExists = true
							showRepackDialog()
						}
					}()
				}))
			} else {
				// STATE 2: REPACK
				content.Add(widget.NewLabel("Step 2: Modify & Repack"))

				// Backup Logic
				backupDir := filepath.Join(settingsPath, BackupDirName)
				_, errBackup := os.Stat(backupDir)
				backupExists := errBackup == nil

				backupUI := container.NewVBox()
				if backupExists {
					backupUI.Add(widget.NewLabel("Backup found."))

					btnRevert := widget.NewButton("Revert to Backup", func() {
						loading := dialog.NewCustom("Restoring Backup...", "Cancel", widget.NewProgressBarInfinite(), w)
						loading.Show()
						go func() {
							// Copy from Settings/Backup -> EchoDataPath
							err := copyRecursive(backupDir, appSettings.EchoVRDataPath)
							loading.Hide()
							if err != nil {
								dialog.ShowError(err, w)
							} else {
								dialog.ShowInformation("Success", "Game files reverted to backup.", w)
							}
						}()
					})
					btnRevert.Importance = widget.WarningImportance
					backupUI.Add(btnRevert)

				} else {
					backupUI.Add(widget.NewLabel("No backup found."))
					backupUI.Add(widget.NewButton("Create Backup", func() {
						loading := dialog.NewCustom("Backing up...", "Cancel", widget.NewProgressBarInfinite(), w)
						loading.Show()
						go func() {
							os.MkdirAll(backupDir, 0755)
							err := copyRecursive(appSettings.EchoVRDataPath, backupDir)
							loading.Hide()
							if err != nil {
								dialog.ShowError(err, w)
							} else {
								dialog.ShowInformation("Backup", "Backup created successfully.", w)
								showRepackDialog()
							}
						}()
					}))
				}
				content.Add(widget.NewCard("Backup", "", backupUI))

				content.Add(widget.NewSeparator())
				content.Add(widget.NewLabel("Ready to Repack changes into game."))
				content.Add(widget.NewButton("REPACK & APPLY", func() {
					loading := dialog.NewCustom("Repacking...", "Cancel", widget.NewProgressBarInfinite(), w)
					loading.Show()

					go func() {
						output, err := executeRepackTool(appSettings.EchoVRDataPath)
						loading.Hide()

						// Always print output for debugging
						if output != "" {
							fmt.Println("--- Repack Tool Output ---")
							fmt.Println(output)
							fmt.Println("--------------------------")
						}

						if err != nil {
							dialog.ShowError(err, w)
							return
						}

						dialog.ShowConfirm("Push files?", "Repack complete. Do you want to push files to the game?", func(confirm bool) {
							if confirm {
								pushLoading := dialog.NewCustom("Pushing files...", "Cancel", widget.NewProgressBarInfinite(), w)
								pushLoading.Show()

								go func() {
									err := pushRepackedFiles(appSettings.EchoVRDataPath)
									pushLoading.Hide()
									if err != nil {
										dialog.ShowError(err, w)
									} else {
										dialog.ShowInformation("Success", "Tints repacked and applied to game!", w)
									}
								}()
							}
						}, w)
					}()
				}))
			}
			content.Refresh()
		}

		refreshUI()
		modal.Show()
	}

	btnRepack := widget.NewButton("REPACK", showRepackDialog)

	// --- LIST WIDGET ---
	loadTintToEditor := func(realIdx int) {
		unbindListeners()
		isLoadingEntry = true
		defer func() {
			isLoadingEntry = false
			bindListeners()
		}()

		loadFromTemp()

		selectedListIndex = realIdx
		t := CTint{}
		if err := t.FromCosmeticEntry(currentCList.cosmeticEntries[realIdx]); err != nil {
			return
		}

		nameEntry.SetText(t.DisplayName)
		descEntry.SetText(t.Description)

		toHex := func(r, g, b float32) string {
			ri, gi, bi := int(r*255), int(g*255), int(b*255)
			return fmt.Sprintf("%02X%02X%02X", ri, gi, bi)
		}
		primaryHex.SetText(toHex(t.SecondaryColor_R, t.SecondaryColor_G, t.SecondaryColor_B))
		secondaryHex.SetText(toHex(t.PrimaryColor_R, t.PrimaryColor_G, t.PrimaryColor_B))

		if name, ok := raritySymbolToName[t.Rarity]; ok {
			raritySelect.SetSelected(name)
		} else {
			raritySelect.SetSelected("Default") // Fallback
		}

		thumbIdEntry.SetText(strconv.FormatInt(t.ThumbnailSymbol, 10))
		refreshThumbnail(thumbIdEntry.Text)
	}

	loadTitleToEditor := func(realIdx int) {
		unbindListeners()
		isLoadingEntry = true
		defer func() {
			isLoadingEntry = false
			bindListeners()
		}()

		loadFromTemp()

		selectedListIndex = realIdx
		t := CTitle{}
		if err := t.FromCosmeticEntry(currentCList.cosmeticEntries[realIdx]); err != nil {
			return
		}

		nameEntry.SetText(t.DisplayName)
		descEntry.SetText(t.Description)
		titleStringEntry.SetText(t.TitleString)

		if name, ok := raritySymbolToName[t.Rarity]; ok {
			raritySelect.SetSelected(name)
		} else {
			raritySelect.SetSelected("Default") // Fallback
		}

		thumbIdEntry.SetText(strconv.FormatInt(t.ThumbnailSymbol, 10))
		refreshThumbnail(thumbIdEntry.Text)
	}

	loadEmissiveToEditor := func(realIdx int) {
		unbindListeners()
		isLoadingEntry = true
		defer func() {
			isLoadingEntry = false
			// Re-attach complex listener for colors
			emissiveColorsEntry.OnChanged = onEmissiveColorsChanged
			bindListeners()
		}()

		loadFromTemp()
		selectedListIndex = realIdx
		t := CEmissive{}
		if err := t.FromCosmeticEntry(currentCList.cosmeticEntries[realIdx]); err != nil {
			return
		}

		nameEntry.SetText(t.DisplayName)
		descEntry.SetText(t.Description)

		if name, ok := raritySymbolToName[t.Rarity]; ok {
			raritySelect.SetSelected(name)
		} else {
			raritySelect.SetSelected("Default") // Fallback
		}

		thumbIdEntry.SetText(strconv.FormatInt(t.ThumbnailSymbol, 10))

		emissiveUnk1Entry.SetText(fmt.Sprintf("%f", t.Unk1))
		emissiveUnk2Entry.SetText(fmt.Sprintf("%f", t.Unk2))
		emissiveTexEntry.SetText(strconv.FormatInt(t.TextureSymbol, 10))

		var colorStr strings.Builder
		for _, c := range t.Colors {
			ri, gi, bi := int(c[0]*255), int(c[1]*255), int(c[2]*255)
			colorStr.WriteString(fmt.Sprintf("%02X%02X%02X\n", ri, gi, bi))
		}
		emissiveColorsEntry.SetText(colorStr.String())
		refreshEmissivePreview(t.Colors)

		refreshThumbnail(thumbIdEntry.Text)
	}

	searchEntry := widget.NewEntry()
	searchEntry.PlaceHolder = "Search Tints..."

	searchEntryTitles := widget.NewEntry()
	searchEntryTitles.PlaceHolder = "Search Titles..."

	searchEntryEmissives := widget.NewEntry()
	searchEntryEmissives.PlaceHolder = "Search Emissives..."

	tintList := widget.NewList(
		func() int { return len(filteredIndices) },
		func() fyne.CanvasObject { return widget.NewLabel("Template") },
		func(id widget.ListItemID, item fyne.CanvasObject) {
			realIndex := filteredIndices[id]
			entry := currentCList.cosmeticEntries[realIndex]
			dName := string(bytes.TrimRight(entry.cEntry.DisplayNameString[:], "\x00"))
			item.(*widget.Label).SetText(dName)
		},
	)

	titleList := widget.NewList(
		func() int { return len(filteredTitleIndices) },
		func() fyne.CanvasObject { return widget.NewLabel("Template") },
		func(id widget.ListItemID, item fyne.CanvasObject) {
			realIndex := filteredTitleIndices[id]
			entry := currentCList.cosmeticEntries[realIndex]
			dName := string(bytes.TrimRight(entry.cEntry.DisplayNameString[:], "\x00"))
			item.(*widget.Label).SetText(dName)
		},
	)

	emissiveList := widget.NewList(
		func() int { return len(filteredEmissiveIndices) },
		func() fyne.CanvasObject { return widget.NewLabel("Template") },
		func(id widget.ListItemID, item fyne.CanvasObject) {
			realIndex := filteredEmissiveIndices[id]
			entry := currentCList.cosmeticEntries[realIndex]
			dName := string(bytes.TrimRight(entry.cEntry.DisplayNameString[:], "\x00"))
			item.(*widget.Label).SetText(dName)
		},
	)

	tintList.OnSelected = func(id widget.ListItemID) { loadTintToEditor(filteredIndices[id]) }
	titleList.OnSelected = func(id widget.ListItemID) { loadTitleToEditor(filteredTitleIndices[id]) }
	emissiveList.OnSelected = func(id widget.ListItemID) { loadEmissiveToEditor(filteredEmissiveIndices[id]) }

	loadFanfareToEditor := func(realIdx int) {
		unbindListeners()
		isLoadingEntry = true
		defer func() {
			isLoadingEntry = false
			bindListeners()
		}()

		loadFromTemp()

		selectedListIndex = realIdx
		t := CFanfare{}
		if err := t.FromCosmeticEntry(currentCList.cosmeticEntries[realIdx]); err != nil {
			return
		}

		nameEntry.SetText(t.DisplayName)
		descEntry.SetText(t.Description)

		if name, ok := raritySymbolToName[t.Rarity]; ok {
			raritySelect.SetSelected(name)
		} else {
			raritySelect.SetSelected("Default") // Fallback
		}
		fanfare1Entry.SetText(strconv.FormatUint(uint64(t.Fanfare1ID), 10))
		fanfare2Entry.SetText(strconv.FormatUint(uint64(t.Fanfare2ID), 10))
		thumbIdEntry.SetText(strconv.FormatInt(t.ThumbnailSymbol, 10))
		refreshThumbnail(thumbIdEntry.Text)
	}
	fanfareList.OnSelected = func(id widget.ListItemID) { loadFanfareToEditor(filteredFanfareIndices[id]) }

	refreshTintFilter := func() {
		txt := strings.ToLower(searchEntry.Text)
		filteredIndices = []int{}
		for _, idx := range tintIndices {
			dName := strings.ToLower(string(bytes.TrimRight(currentCList.cosmeticEntries[idx].cEntry.DisplayNameString[:], "\x00")))
			if txt == "" || strings.Contains(dName, txt) {
				filteredIndices = append(filteredIndices, idx)
			}
		}
		tintList.Refresh()
	}
	searchEntry.OnChanged = func(s string) { refreshTintFilter() }

	refreshTitleFilter := func() {
		txt := strings.ToLower(searchEntryTitles.Text)
		filteredTitleIndices = []int{}
		for _, idx := range titleIndices {
			dName := strings.ToLower(string(bytes.TrimRight(currentCList.cosmeticEntries[idx].cEntry.DisplayNameString[:], "\x00")))
			if txt == "" || strings.Contains(dName, txt) {
				filteredTitleIndices = append(filteredTitleIndices, idx)
			}
		}
		titleList.Refresh()
	}
	searchEntryTitles.OnChanged = func(s string) { refreshTitleFilter() }

	refreshEmissiveFilter := func() {
		txt := strings.ToLower(searchEntryEmissives.Text)
		filteredEmissiveIndices = []int{}
		for _, idx := range emissiveIndices {
			dName := strings.ToLower(string(bytes.TrimRight(currentCList.cosmeticEntries[idx].cEntry.DisplayNameString[:], "\x00")))
			if txt == "" || strings.Contains(dName, txt) {
				filteredEmissiveIndices = append(filteredEmissiveIndices, idx)
			}
		}
		emissiveList.Refresh()
	}
	searchEntryEmissives.OnChanged = func(s string) { refreshEmissiveFilter() }

	refreshFanfareFilter := func() {
		txt := strings.ToLower(searchEntryFanfares.Text)
		filteredFanfareIndices = []int{}
		for _, idx := range fanfareIndices {
			dName := strings.ToLower(string(bytes.TrimRight(currentCList.cosmeticEntries[idx].cEntry.DisplayNameString[:], "\x00")))
			if txt == "" || strings.Contains(dName, txt) {
				filteredFanfareIndices = append(filteredFanfareIndices, idx)
			}
		}
		fanfareList.Refresh()
	}
	searchEntryFanfares.OnChanged = func(s string) { refreshFanfareFilter() }

	refreshIndices := func() {
		tintIndices = []int{}
		titleIndices = []int{}
		emissiveIndices = []int{}
		fanfareIndices = []int{}
		tintSymbol := int64(ToSymbol("tint"))
		titleSymbol := int64(ToSymbol("title"))
		emissiveSymbol := int64(ToSymbol("emissive"))

		for i, e := range currentCList.cosmeticEntries {
			// Identify fanfares first, as they can have any cosmetic type symbol
			if e.cEntry.WWiseSoundBankID2 != 0 {
				fanfareIndices = append(fanfareIndices, i)
				continue // Go to next item so it's not added to other lists
			}

			switch e.cEntry.CosmeticTypeSymbol {
			case tintSymbol:
				isEmissive := strings.Contains(strings.ToLower(string(bytes.TrimRight(e.cEntry.InternalNameString[:], "\x00"))), "emissive") ||
					e.cEntry.TextureSymbol != -1 ||
					len(e.cEntryExtData) != 24 ||
					e.cEntry.EmissiveUnk1 != 0 ||
					e.cEntry.EmissiveUnk2 != 0

				if !isEmissive {
					tintIndices = append(tintIndices, i)
				} else {
					emissiveIndices = append(emissiveIndices, i)
				}
			case emissiveSymbol:
				emissiveIndices = append(emissiveIndices, i)
			case titleSymbol:
				titleIndices = append(titleIndices, i)
			}
		}
		refreshTintFilter()
		refreshTitleFilter()
		refreshEmissiveFilter()
		refreshFanfareFilter()
	}

	initData := func() {
		dataCopy := make([]byte, len(embeddedOriginal))
		copy(dataCopy, embeddedOriginal)
		os.WriteFile(tempFilePath, dataCopy, 0644)
		loadFromTemp()
		defer func() {
			if r := recover(); r != nil {
			}
		}()
		refreshIndices()
		statusLabel.SetText("Loaded. Temp file created.")
	}

	// --- EXTERNAL FILE LOADING ---
	loadExternalFile := func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if reader == nil {
				return
			}
			defer reader.Close()
			data, err := io.ReadAll(reader)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if _, err := bytesToCosmeticList(data); err != nil {
				dialog.ShowError(fmt.Errorf("Invalid file format: %v", err), w)
				return
			}
			os.WriteFile(tempFilePath, data, 0644)
			loadFromTemp()
			refreshIndices()
			statusLabel.SetText("Loaded External: " + reader.URI().Name())
			dialog.ShowInformation("Success", "Loaded "+reader.URI().Name(), w)
		}, w)
	}

	// --- SETTINGS UI ---
	lblAssets := widget.NewLabel(appSettings.AssetsPath)
	lblTexOut := widget.NewLabel(appSettings.TextureOutPath)
	lblMetaOut := widget.NewLabel(appSettings.MetadataOutPath)
	lblDataPath := widget.NewLabel(appSettings.EchoVRDataPath)

	if appSettings.AssetsPath == "" {
		lblAssets.SetText("Not Set")
	}
	if appSettings.TextureOutPath == "" {
		lblTexOut.SetText("Not Set")
	}
	if appSettings.MetadataOutPath == "" {
		lblMetaOut.SetText("Not Set")
	}

	btnSetTexOut := widget.NewButton("Browse", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if uri != nil {
				appSettings.TextureOutPath = uri.Path()
				lblTexOut.SetText(appSettings.TextureOutPath)
				saveSettings()
			}
		}, w)
	})

	btnSetMetaOut := widget.NewButton("Browse", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if uri != nil {
				appSettings.MetadataOutPath = uri.Path()
				lblMetaOut.SetText(appSettings.MetadataOutPath)
				saveSettings()
			}
		}, w)
	})

	btnSetDataPath := widget.NewButton("Browse", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if uri != nil {
				appSettings.EchoVRDataPath = uri.Path()
				lblDataPath.SetText(appSettings.EchoVRDataPath)
				saveSettings()
			}
		}, w)
	})

	showSettings := func() {
		content := container.NewVBox(
			widget.NewLabel("Settings"), widget.NewSeparator(),
			widget.NewLabel("Thumbnails are downloaded automatically."),
			widget.NewSeparator(),
			widget.NewLabel("EchoVR Data Path:"), container.NewBorder(nil, nil, nil, btnSetDataPath, lblDataPath),
			widget.NewSeparator(),
			widget.NewLabel("Output Texture Folder (DDS):"), container.NewBorder(nil, nil, nil, btnSetTexOut, lblTexOut),
			widget.NewLabel("Output Metadata Folder (Hex):"), container.NewBorder(nil, nil, nil, btnSetMetaOut, lblMetaOut),
		)
		dialog.NewCustom("Settings", "Close", content, w).Show()
	}
	btnSettings := widget.NewButton("Settings", showSettings)
	btnLoadFile := widget.NewButton("Load File", loadExternalFile)

	var btnChooseMode *widget.Button
	btnChooseMode = widget.NewButton("Choose Mode (Current: "+appSettings.Mode+")", func() {
		dialog.ShowCustom("Select Mode", "Cancel", container.NewVBox(
			widget.NewButton("PCVR", func() {
				updateMode("PCVR")
				btnChooseMode.SetText("Choose Mode (Current: PCVR)")
				saveSettings()
				w.Content().Refresh()
			}),
			widget.NewButton("Quest", func() {
				updateMode("Quest")
				btnChooseMode.SetText("Choose Mode (Current: Quest)")
				saveSettings()
				w.Content().Refresh()
			}),
		), w)
	})

	saveBtn := widget.NewButton("SAVE DATA FILE (Cosmetics)", func() {
		b, err := cosmeticListToBytes(currentCList)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}

		// Ensure directories exist
		settingsPath := getSettingsDir()
		absInputDir := filepath.Join(settingsPath, InputDirName)
		tintDir := filepath.Join(absInputDir, TintFolder)
		if err := os.MkdirAll(tintDir, 0755); err != nil {
			dialog.ShowError(err, w)
			return
		}

		targetFile := filepath.Join(tintDir, TintFileName)
		if err := os.WriteFile(targetFile, b, 0644); err != nil {
			dialog.ShowError(err, w)
			return
		}

		dialog.ShowInformation("Saved", "Cosmetic List Saved to:\n"+targetFile, w)
	})

	// --- LAYOUT ---
	// Shared editor components

	colorForm := widget.NewForm(
		widget.NewFormItem("Main", primaryHex),
		widget.NewFormItem("Secondary", secondaryHex),
	)

	previewLabel := widget.NewLabel("Preview")
	previewContainer := container.NewCenter(thumbContainer)

	// Editor panel (right side)
	// This will be dynamically adjusted based on the selected tab
	editorContainer := container.NewVBox(
		previewLabel,
		previewContainer,
		emissivePreviewLabel,
		emissivePreviewWrapper,
		btnGenThumb,
		widget.NewSeparator(),
		widget.NewLabel("Info"), nameEntry, descEntry, thumbIdEntry,
		widget.NewForm(widget.NewFormItem("Rarity", raritySelect)),
		widget.NewSeparator(),
	)

	tintEditor := container.NewVBox(widget.NewLabel("Colors"), colorForm)
	titleEditor := container.NewVBox(widget.NewForm(widget.NewFormItem("Title Text", titleStringEntry)))
	emissiveEditor := container.NewVBox(
		widget.NewLabel("Colors (Hex, one per line)"),
		container.NewGridWrap(fyne.NewSize(400, 150), emissiveColorsEntry),
	)
	fanfareEditor := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Explosion Sound ID", fanfare1Entry),
			widget.NewFormItem("Fanfare Sound ID", fanfare2Entry),
		),
	)

	// Add a placeholder that will be swapped
	editorContainer.Add(container.NewVBox())

	right := container.NewVBox(
		editorContainer,
		layout.NewSpacer(),
		saveBtn,
		statusLabel,
	)

	tintTabContent := container.NewBorder(container.NewVBox(searchEntry), nil, nil, nil, tintList)
	titleTabContent := container.NewBorder(container.NewVBox(searchEntryTitles), nil, nil, nil, titleList)
	emissiveTabContent := container.NewBorder(container.NewVBox(searchEntryEmissives), nil, nil, nil, emissiveList)
	fanfareTabContent := container.NewBorder(container.NewVBox(searchEntryFanfares), nil, nil, nil, fanfareList)

	tabs = container.NewAppTabs(
		container.NewTabItem("Tints", tintTabContent),
		container.NewTabItem("Titles", titleTabContent),
		container.NewTabItem("Emissives", emissiveTabContent),
		container.NewTabItem("Fanfares", fanfareTabContent),
	)

	tabs.OnSelected = func(tab *container.TabItem) {
		// Hide components that are not always visible
		previewLabel.Hide()
		previewContainer.Hide()
		emissivePreviewLabel.Hide()
		emissivePreviewWrapper.Hide()
		btnGenThumb.Hide()

		// Determine which editor to show
		var specificEditor fyne.CanvasObject
		switch tab.Text {
		case "Tints":
			specificEditor = tintEditor
			previewLabel.Show()
			previewContainer.Show()
			btnGenThumb.Show()
		case "Titles":
			specificEditor = titleEditor
		case "Emissives":
			specificEditor = emissiveEditor
			emissivePreviewLabel.Show()
			emissivePreviewWrapper.Show()
		case "Fanfares":
			specificEditor = fanfareEditor
		default:
			specificEditor = container.NewVBox()
		}
		editorContainer.Objects[len(editorContainer.Objects)-1] = specificEditor
		editorContainer.Refresh()
	}

	// --- Final Layout ---
	topRow := container.NewVBox(
		container.NewGridWithColumns(1, btnRepack),
		container.NewGridWithColumns(3, btnSettings, btnLoadFile, btnChooseMode),
	)

	left := container.NewBorder(topRow, nil, nil, nil, tabs)

	w.SetContent(container.NewHSplit(left, container.NewPadded(container.NewVScroll(right))))

	go func() {
		time.Sleep(200 * time.Millisecond)
		if _, err := os.Stat(tempFilePath); err == nil {
			loadFromTemp()
			refreshIndices()
			statusLabel.SetText("Resumed from autosave.")
		} else {
			initData()
		}
		if selectedListIndex != -1 {
			refreshThumbnail(thumbIdEntry.Text)
		}
	}()

	w.ShowAndRun()
}

// --- THUMBNAIL RENAMING LOGIC ---
func toUint64Hex(name string) (string, bool) {
	// Handle 0x prefix removal
	if len(name) > 2 && strings.ToLower(name[:2]) == "0x" {
		return name[2:], true
	}
	// Try parsing as decimal int64
	if val, err := strconv.ParseInt(name, 10, 64); err == nil {
		// Convert to uint64 hex (handles two's complement for negatives)
		return fmt.Sprintf("%016x", uint64(val)), true
	}
	return "", false
}

func renameThumbnails(root string) {
	var paths []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && path != root {
			paths = append(paths, path)
		}
		return nil
	})

	// Sort by length descending to rename children before parents (bottom-up)
	sort.Slice(paths, func(i, j int) bool {
		return len(paths[i]) > len(paths[j])
	})

	for _, path := range paths {
		dir := filepath.Dir(path)
		base := filepath.Base(path)
		ext := filepath.Ext(base)
		nameNoExt := strings.TrimSuffix(base, ext)

		if hexName, ok := toUint64Hex(nameNoExt); ok {
			newPath := filepath.Join(dir, hexName+ext)
			os.Rename(path, newPath)
		}
	}
}

// --- THUMBNAIL DOWNLOADER ---
func downloadAndExtractThumbnails() {
	url := "https://api.github.com/repos/heisthecat31/EchoVR-Tint-Editor/releases/tags/Editor"
	settingsPath := getSettingsDir()
	os.MkdirAll(settingsPath, 0755)

	zipName := filepath.Join(settingsPath, "thumbnail.zip")
	targetDir := filepath.Join(settingsPath, "thumbnail")

	// Check if already extracted
	if _, err := os.Stat(targetDir); err == nil {
		fmt.Println("Thumbnails already present.")
		return
	}

	fmt.Println("Fetching release info...")
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Failed to fetch release info:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Failed to fetch release info: HTTP %d\n", resp.StatusCode)
		return
	}

	var release struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadUrl string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		fmt.Println("Failed to decode release info:", err)
		return
	}

	downloadUrl := ""
	for _, asset := range release.Assets {
		if asset.Name == "thumbnail.zip" {
			downloadUrl = asset.BrowserDownloadUrl
			break
		}
	}

	if downloadUrl == "" {
		fmt.Println("thumbnail.zip not found in release assets")
		return
	}

	fmt.Println("Downloading thumbnails from:", downloadUrl)
	resp, err = http.Get(downloadUrl)
	if err != nil {
		fmt.Println("Failed to download thumbnails:", err)
		return
	}
	defer resp.Body.Close()

	out, err := os.Create(zipName)
	if err != nil {
		fmt.Println("Failed to create zip file:", err)
		return
	}

	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		fmt.Println("Failed to save zip:", err)
		return
	}

	// Extract
	r, err := zip.OpenReader(zipName)
	if err != nil {
		fmt.Println("Failed to open zip:", err)
		return
	}
	defer r.Close()

	os.MkdirAll(targetDir, 0755)

	for _, f := range r.File {
		fpath := filepath.Join(targetDir, f.Name)

		if !strings.HasPrefix(fpath, filepath.Clean(targetDir)+string(os.PathSeparator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			continue
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			continue
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
	}

	os.Remove(zipName)

	// Run renaming logic on extracted thumbnails
	renameThumbnails(targetDir)

	fmt.Println("Thumbnails downloaded and extracted.")
}

// --- METADATA WRITER ---
func writeMetadata(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	padding := bytes.Repeat([]byte{0xFF}, 192)
	f.Write(padding)

	var tail []byte
	if appSettings.Mode == "Quest" {
		tail = []byte{
			0x01, 0x00, 0x00, 0x00,
			0x80, 0x00, 0x00, 0x00,
			0x80, 0x00, 0x00, 0x00,
			0x01, 0x00, 0x00, 0x00,
			0x01, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
			0x63, 0x00, 0x00, 0x00,
			0x01, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
			0x80, 0x00, 0x00, 0x00,
			0x80, 0x00, 0x00, 0x00,
			0x01, 0x00, 0x00, 0x00,
			0x40, 0x1E, 0x00, 0x00,
			0x00, 0x40, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
		}
	} else {
		tail = []byte{
			0x01, 0x00, 0x00, 0x00,
			0x80, 0x00, 0x00, 0x00,
			0x80, 0x00, 0x00, 0x00,
			0x01, 0x00, 0x00, 0x00,
			0x01, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
			0x63, 0x00, 0x00, 0x00,
			0x01, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
			0x80, 0x00, 0x00, 0x00,
			0x80, 0x00, 0x00, 0x00,
			0x01, 0x00, 0x00, 0x00,
			0x04, 0x56, 0x00, 0x00,
			0x00, 0x40, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00,
		}
	}

	f.Write(tail)
	return nil
}

// --- STANDARD HELPERS ---
func loadSettings() {
	appSettings.TextureOutPath = ""
	appSettings.MetadataOutPath = ""
	file, err := os.ReadFile(settingsFile)
	if err == nil {
		json.Unmarshal(file, &appSettings)
	}
}
func saveSettings() {
	b, _ := json.MarshalIndent(appSettings, "", "  ")
	os.WriteFile(settingsFile, b, 0644)
}

type cosmeticList struct {
	_               [8]byte
	listSize        uint64
	_               [12]byte
	unk1            uint32
	_               [8]byte
	listCount       uint64
	listCount2      uint64
	cosmeticEntries []cosmeticEntry
}

func bytesToCosmeticList(b []byte) (cosmeticList, error) {
	var cList cosmeticList
	if len(b) < 56 {
		return cList, fmt.Errorf("file too small")
	}
	cList.listSize = binary.LittleEndian.Uint64(b[8:16])
	cList.unk1 = binary.LittleEndian.Uint32(b[28:32])
	cList.listCount = binary.LittleEndian.Uint64(b[40:48])
	cList.listCount2 = binary.LittleEndian.Uint64(b[48:56])
	headerSize := 56 + int(cList.listCount)*664
	if len(b) < headerSize {
		return cList, fmt.Errorf("header incomplete")
	}
	extData := b[headerSize:]
	cList.cosmeticEntries = make([]cosmeticEntry, cList.listCount)
	reader := bytes.NewReader(b[56:headerSize])
	for i := 0; i < int(cList.listCount); i++ {
		binary.Read(reader, binary.LittleEndian, &cList.cosmeticEntries[i].cEntry)
		sz := int(cList.cosmeticEntries[i].cEntry.ImageListingEntrySize + cList.cosmeticEntries[i].cEntry.OtherEntrySize)
		if sz > 0 && sz <= len(extData) {
			cList.cosmeticEntries[i].cEntryExtData = make([]byte, sz)
			copy(cList.cosmeticEntries[i].cEntryExtData, extData[:sz])
			extData = extData[sz:]
		}
	}
	return cList, nil
}
func cosmeticListToBytes(cList cosmeticList) ([]byte, error) {
	buf, _ := binary.Append(nil, binary.LittleEndian, [8]byte{})
	buf, _ = binary.Append(buf, binary.LittleEndian, cList.listSize)
	buf, _ = binary.Append(buf, binary.LittleEndian, [12]byte{})
	buf, _ = binary.Append(buf, binary.LittleEndian, cList.unk1)
	buf, _ = binary.Append(buf, binary.LittleEndian, [8]byte{})
	buf, _ = binary.Append(buf, binary.LittleEndian, cList.listCount)
	buf, _ = binary.Append(buf, binary.LittleEndian, cList.listCount2)
	for i := 0; i < len(cList.cosmeticEntries); i++ {
		buf, _ = binary.Append(buf, binary.LittleEndian, cList.cosmeticEntries[i].cEntry)
	}
	for i := 0; i < len(cList.cosmeticEntries); i++ {
		buf, _ = binary.Append(buf, binary.LittleEndian, cList.cosmeticEntries[i].cEntryExtData)
	}
	return buf, nil
}
