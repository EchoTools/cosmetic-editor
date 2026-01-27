package main

//go:generate rsrc -ico icon.ico

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"math"
	"os"
	"path/filepath"
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
//go:embed 0x43934c379cf1e366_original
var embeddedOriginal []byte

//go:embed icon.ico
var embeddedIcon []byte

//go:embed template_thumb.png
var embeddedTemplate []byte

// --- SETTINGS STRUCT ---
type AppSettings struct {
	AssetsPath      string `json:"assets_path"`
	TextureOutPath  string `json:"texture_out_path"`
	MetadataOutPath string `json:"metadata_out_path"`
}

// Global State
var (
	currentCList      cosmeticList
	tintIndices       []int
	filteredIndices   []int
	selectedListIndex int = -1

	// Flags
	isLoadingEntry bool

	// File Paths
	tempFilePath = "temp_autosave.dat"
	settingsFile = "settings.json"
	appSettings  AppSettings
)

func main() {
	os.Setenv("FYNE_GL_VERSION", "2.1")

	// 1. SETUP: Settings & Crash Recovery
	loadSettings()
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("CRITICAL ERROR:", r)
			time.Sleep(10 * time.Second)
		}
	}()

	a := app.New()
	a.SetIcon(fyne.NewStaticResource("icon.ico", embeddedIcon))

	w := a.NewWindow("Cosmetic Tint Editor (Safe Mode)")
	w.Resize(fyne.NewSize(1100, 750))

	// --- UI DEFINITIONS ---
	statusLabel := widget.NewLabel("Status: Initializing...")

	nameEntry := widget.NewEntry()
	descEntry := widget.NewEntry()
	thumbIdEntry := widget.NewEntry()

	thumbImage := canvas.NewImageFromResource(nil)
	thumbImage.FillMode = canvas.ImageFillContain
	thumbImage.SetMinSize(fyne.NewSize(150, 150))
	thumbContainer := container.NewMax(
		canvas.NewRectangle(color.RGBA{30, 30, 30, 255}),
		thumbImage,
	)

	// HEX ENTRIES
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
	applyChange := func(modifier func(*CTint)) {
		if isLoadingEntry {
			return
		}
		if selectedListIndex == -1 {
			return
		}

		idx := selectedListIndex
		t := CTint{}
		if err := t.FromCosmeticEntry(currentCList.cosmeticEntries[idx]); err != nil {
			return
		}

		modifier(&t)

		newEntry, err := t.ToCosmeticEntry()
		if err != nil {
			return
		}

		// Fix Size Difference
		oldSize := len(currentCList.cosmeticEntries[idx].cEntryExtData)
		newSize := len(newEntry.cEntryExtData)
		currentCList.listSize = uint64(int64(currentCList.listSize) + int64(newSize - oldSize))
		currentCList.cosmeticEntries[idx] = newEntry

		saveToTemp()
	}

	// --- LISTENERS ---
	onName := func(s string) { applyChange(func(t *CTint) { t.DisplayName = s }) }
	onDesc := func(s string) { applyChange(func(t *CTint) { t.Description = s }) }

	refreshThumbnail := func(idStr string) {
		if idStr == "" {
			return
		}
		found := false

		// 1. Check Output Path (Texture Folder)
		if appSettings.TextureOutPath != "" {
			tintedPath := filepath.Join(appSettings.TextureOutPath, idStr)
			if _, err := os.Stat(tintedPath); err == nil {
				found = true
			}
		}

		// 2. Check Assets Path (Original .png)
		if appSettings.AssetsPath != "" {
			originalPath := filepath.Join(appSettings.AssetsPath, idStr+".png")
			if _, err := os.Stat(originalPath); err == nil {
				thumbImage.File = originalPath
				thumbImage.Refresh()
				return
			}
		}

		if !found {
			thumbImage.File = ""
			thumbImage.Refresh()
		}
	}

	onThumb := func(s string) {
		refreshThumbnail(s)
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			applyChange(func(t *CTint) { t.ThumbnailSymbol = id })
		}
	}

	// Helper to handle hex input
	onHexChange := func(isPrimary bool) func(string) {
		return func(s string) {
			s = strings.TrimPrefix(s, "#")
			if len(s) != 6 {
				return
			}
			bytes, err := hex.DecodeString(s)
			if err != nil {
				return
			}
			r, g, b := float32(bytes[0])/255.0, float32(bytes[1])/255.0, float32(bytes[2])/255.0

			applyChange(func(t *CTint) {
				if isPrimary {
					t.PrimaryColor_R, t.PrimaryColor_G, t.PrimaryColor_B = r, g, b
				} else {
					t.SecondaryColor_R, t.SecondaryColor_G, t.SecondaryColor_B = r, g, b
				}
			})
		}
	}

	unbindListeners := func() {
		nameEntry.OnChanged = nil
		descEntry.OnChanged = nil
		thumbIdEntry.OnChanged = nil
		primaryHex.OnChanged = nil
		secondaryHex.OnChanged = nil
	}

	bindListeners := func() {
		nameEntry.OnChanged = onName
		descEntry.OnChanged = onDesc
		thumbIdEntry.OnChanged = onThumb
		primaryHex.OnChanged = onHexChange(true)
		secondaryHex.OnChanged = onHexChange(false)
	}

	// --- THUMBNAIL GENERATOR ---
	generateAndSaveThumbnail := func() {
		if appSettings.TextureOutPath == "" || appSettings.MetadataOutPath == "" {
			dialog.ShowInformation("Error", "Set both Output Texture and Output Metadata folders in settings.", w)
			return
		}
		if thumbIdEntry.Text == "" {
			dialog.ShowInformation("Error", "No Thumbnail ID set.", w)
			return
		}

		// 1. Load Embedded Template
		img, _, err := image.Decode(bytes.NewReader(embeddedTemplate))
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to decode embedded template: %v", err), w)
			return
		}

		// 2. Colors
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
		srcPrimary := color.RGBA{0x9F, 0x12, 0x13, 0xFF}
		srcSecondary := color.RGBA{0xEC, 0xDB, 0x10, 0xFF}

		isSimilar := func(c1, c2 color.RGBA, threshold float64) bool {
			rDiff := float64(c1.R) - float64(c2.R)
			gDiff := float64(c1.G) - float64(c2.G)
			bDiff := float64(c1.B) - float64(c2.B)
			return math.Sqrt(rDiff*rDiff+gDiff*gDiff+bDiff*bDiff) < threshold
		}

		// 3. Create Tinted Image
		bounds := img.Bounds()
		dst := image.NewRGBA(bounds)

		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				srcC := img.At(x, y)
				r, g, b, a := srcC.RGBA()
				currColor := color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
				finalColor := currColor
				if isSimilar(currColor, srcPrimary, 80.0) {
					finalColor = color.RGBA{cPrim.R, cPrim.G, cPrim.B, currColor.A}
				} else if isSimilar(currColor, srcSecondary, 80.0) {
					finalColor = color.RGBA{cSec.R, cSec.G, cSec.B, currColor.A}
				}
				dst.Set(x, y, finalColor)
			}
		}

		// 4. Save DDS (Texture) - NO EXTENSION
		outDdsPath := filepath.Join(appSettings.TextureOutPath, thumbIdEntry.Text)
		_, err = writeDDS(outDdsPath, dst)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}

		// 5. Save Metadata File - NO EXTENSION
		outMetaPath := filepath.Join(appSettings.MetadataOutPath, thumbIdEntry.Text)
		if err := writeMetadata(outMetaPath); err != nil {
			dialog.ShowError(err, w)
			return
		}

		statusLabel.SetText("Generated Texture & Meta: " + thumbIdEntry.Text)
	}

	btnGenThumb := widget.NewButton("Generate & Save Thumbnail (DDS+Meta)", generateAndSaveThumbnail)

	// --- LIST WIDGET ---
	loadEntryToEditor := func(realIdx int) {
		unbindListeners()
		isLoadingEntry = true

		loadFromTemp()

		selectedListIndex = realIdx
		t := CTint{}
		if err := t.FromCosmeticEntry(currentCList.cosmeticEntries[realIdx]); err != nil {
			isLoadingEntry = false
			bindListeners()
			return
		}

		nameEntry.SetText(t.DisplayName)
		descEntry.SetText(t.Description)

		toHex := func(r, g, b float32) string {
			ri, gi, bi := int(r*255), int(g*255), int(b*255)
			return fmt.Sprintf("%02X%02X%02X", ri, gi, bi)
		}
		primaryHex.SetText(toHex(t.PrimaryColor_R, t.PrimaryColor_G, t.PrimaryColor_B))
		secondaryHex.SetText(toHex(t.SecondaryColor_R, t.SecondaryColor_G, t.SecondaryColor_B))

		thumbIdEntry.SetText(strconv.FormatInt(t.ThumbnailSymbol, 10))

		refreshThumbnail(thumbIdEntry.Text)

		isLoadingEntry = false
		bindListeners()
	}

	searchEntry := widget.NewEntry()
	searchEntry.PlaceHolder = "Search..."

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
	tintList.OnSelected = func(id widget.ListItemID) { loadEntryToEditor(filteredIndices[id]) }

	refreshFilter := func() {
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
	searchEntry.OnChanged = func(s string) { refreshFilter() }

	refreshIndices := func() {
		tintIndices = []int{}
		tintSymbol := int64(ToSymbol("tint"))
		for i, e := range currentCList.cosmeticEntries {
			if e.cEntry.CosmeticTypeSymbol == tintSymbol {
				tintIndices = append(tintIndices, i)
			}
		}
		refreshFilter()
	}

	initData := func() {
		dataCopy := make([]byte, len(embeddedOriginal))
		copy(dataCopy, embeddedOriginal)
		os.WriteFile(tempFilePath, dataCopy, 0644)
		loadFromTemp()
		defer func() { if r := recover(); r != nil {} }()
		refreshIndices()
		statusLabel.SetText(fmt.Sprintf("Loaded. Temp file created."))
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

	if appSettings.AssetsPath == "" {
		lblAssets.SetText("Not Set")
	}
	if appSettings.TextureOutPath == "" {
		lblTexOut.SetText("Not Set")
	}
	if appSettings.MetadataOutPath == "" {
		lblMetaOut.SetText("Not Set")
	}

	btnSetAssets := widget.NewButton("Browse", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if uri != nil {
				appSettings.AssetsPath = uri.Path()
				lblAssets.SetText(appSettings.AssetsPath)
				saveSettings()
				if selectedListIndex != -1 {
					refreshThumbnail(thumbIdEntry.Text)
				}
			}
		}, w)
	})

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

	showSettings := func() {
		content := container.NewVBox(
			widget.NewLabel("Settings"), widget.NewSeparator(),
			widget.NewLabel("Original Assets (For View):"), container.NewBorder(nil, nil, nil, btnSetAssets, lblAssets),
			widget.NewSeparator(),
			widget.NewLabel("Output Texture Folder (DDS):"), container.NewBorder(nil, nil, nil, btnSetTexOut, lblTexOut),
			widget.NewLabel("Output Metadata Folder (Hex):"), container.NewBorder(nil, nil, nil, btnSetMetaOut, lblMetaOut),
		)
		dialog.NewCustom("Settings", "Close", content, w).Show()
	}
	btnSettings := widget.NewButton("Settings", showSettings)
	btnLoadFile := widget.NewButton("Load File", loadExternalFile)

	saveBtn := widget.NewButton("SAVE DATA FILE (Cosmetics)", func() {
		b, err := cosmeticListToBytes(currentCList)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err == nil && writer != nil {
				writer.Write(b)
				writer.Close()
				os.Remove(tempFilePath)
				initData()
				dialog.ShowInformation("Saved", "Cosmetic List Saved.", w)
			}
		}, w)
	})

	// --- LAYOUT ---
	colorForm := widget.NewForm(
		widget.NewFormItem("Primary Hex", primaryHex),
		widget.NewFormItem("Secondary Hex", secondaryHex),
	)

	topLeft := container.NewGridWithColumns(2, btnSettings, btnLoadFile)
	left := container.NewBorder(container.NewVBox(topLeft, widget.NewSeparator(), searchEntry), nil, nil, nil, tintList)

	right := container.NewVBox(
		widget.NewLabel("Preview"),
		container.NewCenter(thumbContainer),
		btnGenThumb,
		widget.NewSeparator(),
		widget.NewLabel("Info"), nameEntry, descEntry, thumbIdEntry,
		widget.NewSeparator(),
		widget.NewLabel("Colors"), colorForm,
		layout.NewSpacer(), saveBtn, statusLabel,
	)

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

// --- METADATA WRITER ---
func writeMetadata(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	// 1. Padding (FFs) - 192 bytes (One "full line" of 16 bytes removed from 208)
	// User requested "1 full line less" than 208. 208 - 16 = 192.
	padding := bytes.Repeat([]byte{0xFF}, 192)
	f.Write(padding)

	// 2. Exact Static Tail Structure (64 bytes)
	tail := []byte{
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
		// Static values:
		0x80, 0x00, 0x01, 0x00,
		0x00, 0x40, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}

	f.Write(tail)
	return nil
}

// --- DDS WRITER (Uncompressed RGBA8) ---
func writeDDS(filename string, img image.Image) (uint32, error) {
	f, err := os.Create(filename)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	const (
		DDS_MAGIC        = 0x20534444
		DDS_HEADER_SIZE  = 124
		DDSD_CAPS        = 0x1
		DDSD_HEIGHT      = 0x2
		DDSD_WIDTH       = 0x4
		DDSD_PITCH       = 0x8
		DDSD_PIXELFORMAT = 0x1000
		DDPF_ALPHAPIXELS = 0x1
		DDPF_RGB         = 0x40
		DDSCAPS_TEXTURE  = 0x1000
	)

	bounds := img.Bounds()
	width := uint32(bounds.Dx())
	height := uint32(bounds.Dy())
	pitch := width * 4 // 4 bytes per pixel

	// Write Magic (4 bytes)
	binary.Write(f, binary.LittleEndian, uint32(DDS_MAGIC))

	// Write Header (124 bytes)
	binary.Write(f, binary.LittleEndian, uint32(DDS_HEADER_SIZE))
	binary.Write(f, binary.LittleEndian, uint32(DDSD_CAPS|DDSD_HEIGHT|DDSD_WIDTH|DDSD_PITCH|DDSD_PIXELFORMAT))
	binary.Write(f, binary.LittleEndian, height)
	binary.Write(f, binary.LittleEndian, width)
	binary.Write(f, binary.LittleEndian, pitch)
	binary.Write(f, binary.LittleEndian, uint32(0))
	binary.Write(f, binary.LittleEndian, uint32(0))
	for i := 0; i < 11; i++ {
		binary.Write(f, binary.LittleEndian, uint32(0))
	}

	// Pixel Format
	binary.Write(f, binary.LittleEndian, uint32(32))
	binary.Write(f, binary.LittleEndian, uint32(DDPF_RGB|DDPF_ALPHAPIXELS))
	binary.Write(f, binary.LittleEndian, uint32(0))
	binary.Write(f, binary.LittleEndian, uint32(32))
	binary.Write(f, binary.LittleEndian, uint32(0x00FF0000))
	binary.Write(f, binary.LittleEndian, uint32(0x0000FF00))
	binary.Write(f, binary.LittleEndian, uint32(0x000000FF))
	binary.Write(f, binary.LittleEndian, uint32(0xFF000000))

	// Caps
	binary.Write(f, binary.LittleEndian, uint32(DDSCAPS_TEXTURE))
	binary.Write(f, binary.LittleEndian, uint32(0))
	binary.Write(f, binary.LittleEndian, uint32(0))
	binary.Write(f, binary.LittleEndian, uint32(0))
	binary.Write(f, binary.LittleEndian, uint32(0))

	// Write Pixel Data (BGRA)
	drawImg := image.NewRGBA(bounds)
	draw.Draw(drawImg, bounds, img, bounds.Min, draw.Src)

	// Calculate Data Size
	dataSize := uint32(0)
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			r, g, b, a := drawImg.At(x, y).RGBA()
			binary.Write(f, binary.LittleEndian, uint8(b>>8))
			binary.Write(f, binary.LittleEndian, uint8(g>>8))
			binary.Write(f, binary.LittleEndian, uint8(r>>8))
			binary.Write(f, binary.LittleEndian, uint8(a>>8))
			dataSize += 4
		}
	}

	totalSize := 4 + 124 + dataSize // Magic + Header + Data
	return totalSize, nil
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