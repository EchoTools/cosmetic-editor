package tints

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	data "evrCosmeticResearch/Data"
	"fmt"
	"math"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// CTint holds the editable fields for a tint cosmetic entry.
type CTint struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64

	PrimaryColor_R   float32
	PrimaryColor_G   float32
	PrimaryColor_B   float32
	SecondaryColor_R float32
	SecondaryColor_G float32
	SecondaryColor_B float32
}

// ToCosmeticEntry converts a CTint to a raw CosmeticEntry for serialization.
func (c *CTint) ToCosmeticEntry() (data.CosmeticEntry, error) {
	foo := data.CosmeticEntry{}
	foo.CEntry = data.NewCDescriptor()

	foo.CEntry.CosmeticTypeSymbol = int64(data.ToSymbol("tint"))
	foo.CEntry.InternalNameSymbol = int64(data.ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.CEntry.InternalNameSymbol2 = foo.CEntry.InternalNameSymbol

	copy(foo.CEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.CEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.CEntry.DescriptionString[:], []byte(c.Description))

	foo.CEntry.RaritySymbol = c.Rarity
	foo.CEntry.ThumbnailSymbol = c.ThumbnailSymbol

	foo.CEntry.OtherEntrySize = 24
	foo.CEntryExtData = make([]byte, 24)

	binary.LittleEndian.PutUint32(foo.CEntryExtData[0:4], math.Float32bits(c.PrimaryColor_R))
	binary.LittleEndian.PutUint32(foo.CEntryExtData[4:8], math.Float32bits(c.PrimaryColor_G))
	binary.LittleEndian.PutUint32(foo.CEntryExtData[8:12], math.Float32bits(c.PrimaryColor_B))
	binary.LittleEndian.PutUint32(foo.CEntryExtData[12:16], math.Float32bits(c.SecondaryColor_R))
	binary.LittleEndian.PutUint32(foo.CEntryExtData[16:20], math.Float32bits(c.SecondaryColor_G))
	binary.LittleEndian.PutUint32(foo.CEntryExtData[20:24], math.Float32bits(c.SecondaryColor_B))

	return foo, nil
}

// FromCosmeticEntry populates a CTint from a raw CosmeticEntry.
func (c *CTint) FromCosmeticEntry(d data.CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = d.CEntry.RaritySymbol
	c.ThumbnailSymbol = d.CEntry.ThumbnailSymbol

	if len(d.CEntryExtData) >= 24 {
		c.PrimaryColor_R = math.Float32frombits(binary.LittleEndian.Uint32(d.CEntryExtData[0:4]))
		c.PrimaryColor_G = math.Float32frombits(binary.LittleEndian.Uint32(d.CEntryExtData[4:8]))
		c.PrimaryColor_B = math.Float32frombits(binary.LittleEndian.Uint32(d.CEntryExtData[8:12]))
		c.SecondaryColor_R = math.Float32frombits(binary.LittleEndian.Uint32(d.CEntryExtData[12:16]))
		c.SecondaryColor_G = math.Float32frombits(binary.LittleEndian.Uint32(d.CEntryExtData[16:20]))
		c.SecondaryColor_B = math.Float32frombits(binary.LittleEndian.Uint32(d.CEntryExtData[20:24]))
	}

	return nil
}

var (
	tintList    *widget.List
	searchEntry *widget.Entry
)

// SetupUI builds and returns the Fyne canvas object for the Tints tab.
func SetupUI(state *data.AppState) fyne.CanvasObject {
	searchEntry = widget.NewEntry()
	searchEntry.PlaceHolder = "Search Tints..."
	searchEntry.OnChanged = func(s string) {
		RefreshFilter(state, s)
	}

	tintList = widget.NewList(
		func() int {
			return len(state.CategoryFiltered["Tints"])
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			realIdx := state.CategoryFiltered["Tints"][id]
			entry := state.CosmeticList.CosmeticEntries[realIdx]
			dName := string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00"))
			item.(*widget.Label).SetText(dName)
		},
	)

	tintList.OnSelected = func(id widget.ListItemID) {
		realIdx := state.CategoryFiltered["Tints"][id]
		LoadToEditor(state, realIdx)
	}

	content := container.NewBorder(searchEntry, nil, nil, nil, tintList)

	return content
}

// LoadToEditor populates the shared sidebar with the tint at the given CosmeticList index.
func LoadToEditor(state *data.AppState, realIdx int) {
	state.SelectedIndex = realIdx
	state.SelectedCategory = "Tints"
	state.RefreshCurrent = func(s *data.AppState) { LoadToEditor(s, realIdx) }

	t := CTint{}
	if err := t.FromCosmeticEntry(state.CosmeticList.CosmeticEntries[realIdx]); err != nil {
		return
	}

	state.IsLoadingEntry = true
	state.NameEntry.SetText(t.DisplayName)
	state.DescEntry.SetText(t.Description)
	state.ThumbIdEntry.SetText(data.SymbolToHex(t.ThumbnailSymbol))
	state.RaritySelect.SetSelected(state.GetRarityName(t.Rarity))
	state.UpdateSidebarThumbnail(t.ThumbnailSymbol)

	// Note: Original main.go swaps these in the UI display
	pHex := widget.NewEntry()
	pHex.SetText(fmt.Sprintf("%02X%02X%02X", int(t.SecondaryColor_R*255), int(t.SecondaryColor_G*255), int(t.SecondaryColor_B*255)))
	sHex := widget.NewEntry()
	sHex.SetText(fmt.Sprintf("%02X%02X%02X", int(t.PrimaryColor_R*255), int(t.PrimaryColor_G*255), int(t.PrimaryColor_B*255)))

	state.CurrentAssetSymbol = data.SymbolToHex(t.ThumbnailSymbol)

	saveColors := func() {
		if state.IsLoadingEntry {
			return
		}
		entry := &state.CosmeticList.CosmeticEntries[state.SelectedIndex]

		parseAndWrite := func(hexStr string, offset int) {
			hexStr = strings.TrimPrefix(hexStr, "#")
			b, err := hex.DecodeString(hexStr)
			if err != nil || len(b) < 3 {
				return
			}

			r := float32(b[0]) / 255.0
			g := float32(b[1]) / 255.0
			bl := float32(b[2]) / 255.0

			if len(entry.CEntryExtData) < offset+12 {
				return
			}
			binary.LittleEndian.PutUint32(entry.CEntryExtData[offset:offset+4], math.Float32bits(r))
			binary.LittleEndian.PutUint32(entry.CEntryExtData[offset+4:offset+8], math.Float32bits(g))
			binary.LittleEndian.PutUint32(entry.CEntryExtData[offset+8:offset+12], math.Float32bits(bl))
		}

		// Swap back when saving: UI pHex is Secondary, sHex is Primary
		parseAndWrite(sHex.Text, 0)
		parseAndWrite(pHex.Text, 12)
	}
	pHex.OnChanged = func(string) { saveColors() }
	sHex.OnChanged = func(string) { saveColors() }

	state.CategoryEditor.Objects = []fyne.CanvasObject{
		widget.NewForm(
			widget.NewFormItem("Primary Color", pHex),
			widget.NewFormItem("Secondary Color", sHex),
		),
	}
	state.CategoryEditor.Refresh()

	state.IsLoadingEntry = false
}

// RefreshFilter re-filters the tint list to entries whose display name contains query.
func RefreshFilter(state *data.AppState, query string) {
	query = strings.ToLower(query)
	state.CategoryFiltered["Tints"] = []int{}
	for _, idx := range state.CategoryIndices["Tints"] {
		entry := state.CosmeticList.CosmeticEntries[idx]
		dName := strings.ToLower(string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00")))
		if query == "" || strings.Contains(dName, query) {
			state.CategoryFiltered["Tints"] = append(state.CategoryFiltered["Tints"], idx)
		}
	}
	if tintList != nil {
		tintList.Refresh()
	}
}
