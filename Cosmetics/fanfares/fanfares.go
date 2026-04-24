package fanfares

import (
	"bytes"
	"evrCosmeticResearch/Data"
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// CFanfare holds the editable fields for a fanfare cosmetic entry.
type CFanfare struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64
	Fanfare1ID      uint32
	Fanfare2ID      uint32
}

// ToCosmeticEntry converts a CFanfare to a raw CosmeticEntry for serialization.
func (c *CFanfare) ToCosmeticEntry() (data.CosmeticEntry, error) {
	foo := data.CosmeticEntry{}
	foo.CEntry = data.NewCDescriptor()

	foo.CEntry.CosmeticTypeSymbol = int64(data.ToSymbol("tint")) // Fanfares use tint as base
	foo.CEntry.InternalNameSymbol = int64(data.ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.CEntry.InternalNameSymbol2 = foo.CEntry.InternalNameSymbol

	copy(foo.CEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.CEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.CEntry.DescriptionString[:], []byte(c.Description))

	foo.CEntry.RaritySymbol = c.Rarity
	foo.CEntry.WWiseSoundBankID1 = c.Fanfare1ID
	foo.CEntry.WWiseSoundBankID2 = c.Fanfare2ID
	foo.CEntry.ThumbnailSymbol = c.ThumbnailSymbol

	return foo, nil
}

// FromCosmeticEntry populates a CFanfare from a raw CosmeticEntry.
func (c *CFanfare) FromCosmeticEntry(d data.CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = d.CEntry.RaritySymbol
	c.Fanfare1ID = d.CEntry.WWiseSoundBankID1
	c.Fanfare2ID = d.CEntry.WWiseSoundBankID2
	c.ThumbnailSymbol = d.CEntry.ThumbnailSymbol
	return nil
}

var (
	fanfareList *widget.List
	searchEntry *widget.Entry
)

// SetupUI builds and returns the Fyne canvas object for the Fanfares tab.
func SetupUI(state *data.AppState) fyne.CanvasObject {
	searchEntry = widget.NewEntry()
	searchEntry.PlaceHolder = "Search Fanfares..."
	searchEntry.OnChanged = func(s string) {
		RefreshFilter(state, s)
	}

	fanfareList = widget.NewList(
		func() int {
			return len(state.CategoryFiltered["Fanfares"])
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			realIdx := state.CategoryFiltered["Fanfares"][id]
			entry := state.CosmeticList.CosmeticEntries[realIdx]
			dName := string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00"))
			item.(*widget.Label).SetText(dName)
		},
	)

	fanfareList.OnSelected = func(id widget.ListItemID) {
		realIdx := state.CategoryFiltered["Fanfares"][id]
		LoadToEditor(state, realIdx)
	}

	content := container.NewBorder(searchEntry, nil, nil, nil, fanfareList)

	return content
}

// LoadToEditor populates the shared sidebar with the fanfare at the given CosmeticList index.
func LoadToEditor(state *data.AppState, realIdx int) {
	state.SelectedIndex = realIdx
	state.SelectedCategory = "Fanfares"

	t := CFanfare{}
	if err := t.FromCosmeticEntry(state.CosmeticList.CosmeticEntries[realIdx]); err != nil {
		return
	}

	state.IsLoadingEntry = true
	state.NameEntry.SetText(t.DisplayName)
	state.DescEntry.SetText(t.Description)
	state.RaritySelect.SetSelected(state.GetRarityName(t.Rarity))

	if state.ThumbIdItem != nil {
		state.ThumbIdItem.Widget.Show()
	}
	state.ThumbIdEntry.SetText(data.SymbolToHex(t.ThumbnailSymbol))
	state.UpdateSidebarThumbnail(t.ThumbnailSymbol)

	f1 := widget.NewEntry()
	f1.SetText(fmt.Sprintf("%d", t.Fanfare1ID))
	f2 := widget.NewEntry()
	f2.SetText(fmt.Sprintf("%d", t.Fanfare2ID))

	saveSounds := func() {
		if state.IsLoadingEntry {
			return
		}
		entry := &state.CosmeticList.CosmeticEntries[state.SelectedIndex]
		v1, _ := strconv.ParseUint(f1.Text, 10, 32)
		entry.CEntry.WWiseSoundBankID1 = uint32(v1)
		v2, _ := strconv.ParseUint(f2.Text, 10, 32)
		entry.CEntry.WWiseSoundBankID2 = uint32(v2)
	}
	f1.OnChanged = func(string) { saveSounds() }
	f2.OnChanged = func(string) { saveSounds() }

	btnSelectPng := widget.NewButtonWithIcon("Set PNG Thumbnail", theme.FileImageIcon(), func() {
		path, err := data.PickFile("PNG Files (*.png)|*.png|All Files (*.*)|*.*")
		if err == nil && path != "" {
			data.HandlePNGThumbnailReplacement(state, state.ThumbIdEntry.Text, path, nil) // Passing nil for button since it's not the same btn
		}
	})

	state.CategoryEditor.Objects = []fyne.CanvasObject{
		container.NewVBox(
			widget.NewForm(
				widget.NewFormItem("Fanfare Sound ID 1", f1),
				widget.NewFormItem("Fanfare Sound ID 2", f2),
			),
			container.NewPadded(btnSelectPng),
		),
	}
	state.CategoryEditor.Refresh()

	state.IsLoadingEntry = false
}

// RefreshFilter re-filters the fanfares list to entries whose display name contains query.
func RefreshFilter(state *data.AppState, query string) {
	query = strings.ToLower(query)
	state.CategoryFiltered["Fanfares"] = []int{}
	for _, idx := range state.CategoryIndices["Fanfares"] {
		entry := state.CosmeticList.CosmeticEntries[idx]
		dName := strings.ToLower(string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00")))
		if query == "" || strings.Contains(dName, query) {
			state.CategoryFiltered["Fanfares"] = append(state.CategoryFiltered["Fanfares"], idx)
		}
	}
	if fanfareList != nil {
		fanfareList.Refresh()
	}
}
