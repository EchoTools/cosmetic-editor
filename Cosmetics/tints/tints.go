package tints

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image/color"
	"math"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	data "github.com/EchoTools/cosmetic-editor/Data"
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

	state.CurrentAssetSymbol = data.SymbolToHex(t.ThumbnailSymbol)

	// The UI has always shown the colour stored second as "Primary" and the
	// first as "Secondary"; that is kept so existing users are not confused.
	// The offsets are into the tint's 24-byte colour data.
	state.CategoryEditor.Objects = []fyne.CanvasObject{
		widget.NewForm(
			widget.NewFormItem("Primary Color", colorField(state, realIdx, 12,
				color.NRGBA{toByte(t.SecondaryColor_R), toByte(t.SecondaryColor_G), toByte(t.SecondaryColor_B), 255})),
			widget.NewFormItem("Secondary Color", colorField(state, realIdx, 0,
				color.NRGBA{toByte(t.PrimaryColor_R), toByte(t.PrimaryColor_G), toByte(t.PrimaryColor_B), 255})),
		),
	}
	state.CategoryEditor.Refresh()

	state.IsLoadingEntry = false
}

func toByte(f float32) uint8 {
	return uint8(math.Max(0, math.Min(255, math.Round(float64(f)*255))))
}

// hexColor parses exactly six hex digits, with or without a leading #.
func hexColor(s string) (color.NRGBA, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return color.NRGBA{}, false
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return color.NRGBA{}, false
	}
	return color.NRGBA{b[0], b[1], b[2], 255}, true
}

// colorField edits one of a tint's two colours: a swatch, a hex entry and a
// colour picker.
//
// The hex entry only takes hex digits and shows an error until it holds all
// six, so a colour that cannot be stored is visible rather than silently
// dropped; typing hex on a headset keyboard is error-prone, which is what the
// picker is for. Edits go to the tint the editor was opened for (idx), not
// whatever is selected when they land, and are saved to disk straight away so
// they survive the app being closed.
func colorField(state *data.AppState, idx, offset int, initial color.NRGBA) fyne.CanvasObject {
	swatch := canvas.NewRectangle(initial)
	swatch.SetMinSize(fyne.NewSize(28, 28))
	swatch.CornerRadius = 4

	entry := widget.NewEntry()
	entry.SetText(fmt.Sprintf("%02X%02X%02X", initial.R, initial.G, initial.B))
	entry.Validator = func(s string) error {
		if _, ok := hexColor(s); !ok {
			return errors.New("enter six hex digits, e.g. FF8800")
		}
		return nil
	}

	write := func(c color.NRGBA) {
		if state.IsLoadingEntry || idx < 0 || idx >= len(state.CosmeticList.CosmeticEntries) {
			return
		}
		ext := state.CosmeticList.CosmeticEntries[idx].CEntryExtData
		if len(ext) < offset+12 {
			return
		}
		for i, v := range []uint8{c.R, c.G, c.B} {
			binary.LittleEndian.PutUint32(ext[offset+4*i:], math.Float32bits(float32(v)/255))
		}
		swatch.FillColor = c
		swatch.Refresh()
		state.AutoSave()
	}

	filtering := false
	entry.OnChanged = func(s string) {
		if filtering {
			return
		}
		// Keep only hex digits, upper-cased, at most six of them.
		clean := make([]byte, 0, 6)
		for _, r := range strings.ToUpper(strings.TrimPrefix(s, "#")) {
			if len(clean) < 6 && strings.ContainsRune("0123456789ABCDEF", r) {
				clean = append(clean, byte(r))
			}
		}
		if string(clean) != s {
			filtering = true
			entry.SetText(string(clean))
			filtering = false
		}
		if c, ok := hexColor(string(clean)); ok {
			write(c)
		}
	}

	pick := widget.NewButtonWithIcon("Pick", theme.ColorPaletteIcon(), func() {
		d := dialog.NewColorPicker("Pick a colour", "", func(c color.Color) {
			n := color.NRGBAModel.Convert(c).(color.NRGBA)
			entry.SetText(fmt.Sprintf("%02X%02X%02X", n.R, n.G, n.B)) // OnChanged saves it
		}, state.Window)
		d.Advanced = true
		if c, ok := hexColor(entry.Text); ok {
			d.SetColor(c)
		}
		d.Show()
	})

	return container.NewBorder(nil, nil, swatch, pick, entry)
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
