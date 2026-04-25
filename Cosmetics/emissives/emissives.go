package emissives

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"github.com/EchoTools/cosmetic-editor/Data"
	"fmt"
	"math"
	"strings"

	"image"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type CEmissive struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64
	TextureSymbol   int64

	Unk1   float32
	Unk2   float32
	Colors [][3]float32
}

func (c *CEmissive) ToCosmeticEntry() (data.CosmeticEntry, error) {
	foo := data.CosmeticEntry{}
	foo.CEntry = data.NewCDescriptor()

	foo.CEntry.CosmeticTypeSymbol = int64(data.ToSymbol("emissive"))
	foo.CEntry.InternalNameSymbol = int64(data.ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.CEntry.InternalNameSymbol2 = foo.CEntry.InternalNameSymbol

	copy(foo.CEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.CEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.CEntry.DescriptionString[:], []byte(c.Description))

	foo.CEntry.RaritySymbol = c.Rarity
	foo.CEntry.ThumbnailSymbol = c.ThumbnailSymbol
	foo.CEntry.TextureSymbol = c.TextureSymbol
	foo.CEntry.EmissiveUnk1 = c.Unk1
	foo.CEntry.EmissiveUnk2 = c.Unk2

	buf := make([]byte, len(c.Colors)*12)
	for i := 0; i < len(c.Colors); i++ {
		binary.LittleEndian.PutUint32(buf[i*12:], math.Float32bits(c.Colors[i][0]))
		binary.LittleEndian.PutUint32(buf[i*12+4:], math.Float32bits(c.Colors[i][1]))
		binary.LittleEndian.PutUint32(buf[i*12+8:], math.Float32bits(c.Colors[i][2]))
	}
	foo.CEntryExtData = buf

	foo.CEntry.ExtDataParamCount = int64(len(c.Colors))
	foo.CEntry.ExtDataParamCount2 = foo.CEntry.ExtDataParamCount
	foo.CEntry.OtherEntrySize = int64(len(c.Colors) * 12)

	return foo, nil
}

func (c *CEmissive) FromCosmeticEntry(d data.CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = d.CEntry.RaritySymbol
	c.ThumbnailSymbol = d.CEntry.ThumbnailSymbol
	c.TextureSymbol = d.CEntry.TextureSymbol
	c.Unk1 = d.CEntry.EmissiveUnk1
	c.Unk2 = d.CEntry.EmissiveUnk2

	if len(d.CEntryExtData) != int(d.CEntry.ExtDataParamCount)*12 {
		return errors.New("invalid extdata size for emissive")
	}
	colors := make([][3]float32, d.CEntry.ExtDataParamCount)
	for i := 0; i < int(d.CEntry.ExtDataParamCount); i++ {
		colors[i][0] = math.Float32frombits(binary.LittleEndian.Uint32(d.CEntryExtData[i*12 : i*12+4]))
		colors[i][1] = math.Float32frombits(binary.LittleEndian.Uint32(d.CEntryExtData[i*12+4 : i*12+8]))
		colors[i][2] = math.Float32frombits(binary.LittleEndian.Uint32(d.CEntryExtData[i*12+8 : i*12+12]))
	}
	c.Colors = colors
	return nil
}

var (
	emissiveList *widget.List
	searchEntry  *widget.Entry
)

func SetupUI(state *data.AppState) fyne.CanvasObject {
	searchEntry = widget.NewEntry()
	searchEntry.PlaceHolder = "Search Emissives..."
	searchEntry.OnChanged = func(s string) {
		RefreshFilter(state, s)
	}

	emissiveList = widget.NewList(
		func() int {
			return len(state.CategoryFiltered["Emissives"])
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			realIdx := state.CategoryFiltered["Emissives"][id]
			entry := state.CosmeticList.CosmeticEntries[realIdx]
			dName := string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00"))
			item.(*widget.Label).SetText(dName)
		},
	)

	emissiveList.OnSelected = func(id widget.ListItemID) {
		realIdx := state.CategoryFiltered["Emissives"][id]
		LoadToEditor(state, realIdx)
	}

	content := container.NewBorder(searchEntry, nil, nil, nil, emissiveList)

	return content
}

func LoadToEditor(state *data.AppState, realIdx int) {
	state.SelectedIndex = realIdx
	state.SelectedCategory = "Emissives"

	t := CEmissive{}
	if err := t.FromCosmeticEntry(state.CosmeticList.CosmeticEntries[realIdx]); err != nil {
		return
	}

	state.IsLoadingEntry = true
	state.NameEntry.SetText(t.DisplayName)
	state.DescEntry.SetText(t.Description)
	state.ThumbIdEntry.SetText(data.SymbolToHex(t.ThumbnailSymbol))
	state.RaritySelect.SetSelected(state.GetRarityName(t.Rarity))
	state.UpdateSidebarThumbnail(t.ThumbnailSymbol)

	state.CurrentAssetSymbol = data.SymbolToHex(t.TextureSymbol)
	state.UpdateMainTexture(t.TextureSymbol)
	
	refreshPreview := func(colors [][3]float32) {
		if len(colors) == 0 {
			state.EmissivePreviewImage.Image = nil
			state.EmissivePreviewImage.Refresh()
			return
		}
		width, height := 256, 256
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		sectionHeight := float64(height) / float64(len(colors))

		for i, c := range colors {
			col := color.RGBA{uint8(c[0] * 255), uint8(c[1] * 255), uint8(c[2] * 255), 255}
			startY := int(float64(i) * sectionHeight)
			endY := int(float64(i+1) * sectionHeight)
			for y := startY; y < endY; y++ {
				for x := 0; x < width; x++ {
					img.Set(x, y, col)
				}
			}
		}
		state.EmissivePreviewImage.Image = img
		state.EmissivePreviewImage.Refresh()
	}

	// Multi-line hex entry like original
	emissiveColorsEntry := widget.NewMultiLineEntry()
	var colorStr strings.Builder
	for _, c := range t.Colors {
		ri, gi, bi := int(c[0]*255), int(c[1]*255), int(c[2]*255)
		colorStr.WriteString(fmt.Sprintf("%02X%02X%02X\n", ri, gi, bi))
	}
	emissiveColorsEntry.SetText(colorStr.String())

	emissiveColorsEntry.OnChanged = func(s string) {
		if state.IsLoadingEntry { return }
		lines := strings.Split(s, "\n")
		var newExtData []byte
		var newColors [][3]float32
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" { continue }
			line = strings.TrimPrefix(line, "#")
			if len(line) != 6 { continue }
			b, err := hex.DecodeString(line)
			if err != nil { continue }
			newExtData = append(newExtData, make([]byte, 12)...)
			off := len(newExtData) - 12
			r, g, bl := float32(b[0])/255.0, float32(b[1])/255.0, float32(b[2])/255.0
			binary.LittleEndian.PutUint32(newExtData[off:], math.Float32bits(r))
			binary.LittleEndian.PutUint32(newExtData[off+4:], math.Float32bits(g))
			binary.LittleEndian.PutUint32(newExtData[off+8:], math.Float32bits(bl))
			newColors = append(newColors, [3]float32{r, g, bl})
		}
		if len(newExtData) > 0 {
			entry := &state.CosmeticList.CosmeticEntries[state.SelectedIndex]
			entry.CEntryExtData = newExtData
			entry.CEntry.ExtDataParamCount = int64(len(newExtData) / 12)
			entry.CEntry.ExtDataParamCount2 = entry.CEntry.ExtDataParamCount
			entry.CEntry.OtherEntrySize = int64(len(newExtData))
			refreshPreview(newColors)
		}
	}

	state.CategoryEditor.Objects = []fyne.CanvasObject{
		widget.NewLabel("Colors (Hex, one per line)"),
		container.NewGridWrap(fyne.NewSize(400, 150), emissiveColorsEntry),
	}
	state.CategoryEditor.Refresh()

	refreshPreview(t.Colors)
	
	state.IsLoadingEntry = false
}

func RefreshFilter(state *data.AppState, query string) {
	query = strings.ToLower(query)
	state.CategoryFiltered["Emissives"] = []int{}
	for _, idx := range state.CategoryIndices["Emissives"] {
		entry := state.CosmeticList.CosmeticEntries[idx]
		dName := strings.ToLower(string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00")))
		if query == "" || strings.Contains(dName, query) {
			state.CategoryFiltered["Emissives"] = append(state.CategoryFiltered["Emissives"], idx)
		}
	}
	if emissiveList != nil {
		emissiveList.Refresh()
	}
}
