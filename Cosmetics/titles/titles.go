package titles

import (
	"bytes"
	"evrCosmeticResearch/Data"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// CTitle holds the editable fields for a title cosmetic entry.
type CTitle struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64

	TitleString string
}

// ToCosmeticEntry converts a CTitle to a raw CosmeticEntry for serialization.
func (c *CTitle) ToCosmeticEntry() (data.CosmeticEntry, error) {
	foo := data.CosmeticEntry{}
	foo.CEntry = data.NewCDescriptor()

	foo.CEntry.CosmeticTypeSymbol = int64(data.ToSymbol("title"))
	foo.CEntry.InternalNameSymbol = int64(data.ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.CEntry.InternalNameSymbol2 = foo.CEntry.InternalNameSymbol

	copy(foo.CEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.CEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.CEntry.DescriptionString[:], []byte(c.Description))

	foo.CEntry.RaritySymbol = c.Rarity
	foo.CEntry.ThumbnailSymbol = c.ThumbnailSymbol
	copy(foo.CEntry.TitleString[:], []byte(c.TitleString))

	return foo, nil
}

// FromCosmeticEntry populates a CTitle from a raw CosmeticEntry.
func (c *CTitle) FromCosmeticEntry(d data.CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = d.CEntry.RaritySymbol
	c.ThumbnailSymbol = d.CEntry.ThumbnailSymbol
	c.TitleString = string(bytes.TrimRight(d.CEntry.TitleString[:], "\x00"))
	return nil
}

var (
	titleList   *widget.List
	searchEntry *widget.Entry
)

// SetupUI builds and returns the Fyne canvas object for the Titles tab.
func SetupUI(state *data.AppState) fyne.CanvasObject {
	searchEntry = widget.NewEntry()
	searchEntry.PlaceHolder = "Search Titles..."
	searchEntry.OnChanged = func(s string) {
		RefreshFilter(state, s)
	}

	titleList = widget.NewList(
		func() int {
			return len(state.CategoryFiltered["Titles"])
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			realIdx := state.CategoryFiltered["Titles"][id]
			entry := state.CosmeticList.CosmeticEntries[realIdx]
			dName := string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00"))
			item.(*widget.Label).SetText(dName)
		},
	)

	titleList.OnSelected = func(id widget.ListItemID) {
		realIdx := state.CategoryFiltered["Titles"][id]
		LoadToEditor(state, realIdx)
	}

	content := container.NewBorder(searchEntry, nil, nil, nil, titleList)

	return content
}

// LoadToEditor populates the shared sidebar with the title at the given CosmeticList index.
func LoadToEditor(state *data.AppState, realIdx int) {
	state.SelectedIndex = realIdx
	state.SelectedCategory = "Titles"

	t := CTitle{}
	if err := t.FromCosmeticEntry(state.CosmeticList.CosmeticEntries[realIdx]); err != nil {
		return
	}

	state.IsLoadingEntry = true
	state.NameEntry.SetText(t.DisplayName)
	state.DescEntry.SetText(t.Description)
	state.RaritySelect.SetSelected(state.GetRarityName(t.Rarity))

	// Hide Thumbnail ID for Titles
	if state.ThumbIdItem != nil {
		state.ThumbIdItem.Widget.Hide()
	}
	state.UpdateSidebarThumbnail(0)

	titleEntry := widget.NewEntry()
	titleEntry.SetText(t.TitleString)
	titleEntry.OnChanged = func(s string) {
		if state.IsLoadingEntry {
			return
		}
		entry := &state.CosmeticList.CosmeticEntries[state.SelectedIndex]
		copy(entry.CEntry.TitleString[:], []byte(s))
	}

	state.CategoryEditor.Objects = []fyne.CanvasObject{
		widget.NewForm(widget.NewFormItem("Title Text", titleEntry)),
	}
	state.CategoryEditor.Refresh()

	state.IsLoadingEntry = false
}

// RefreshFilter re-filters the titles list to entries whose display name contains query.
func RefreshFilter(state *data.AppState, query string) {
	query = strings.ToLower(query)
	state.CategoryFiltered["Titles"] = []int{}
	for _, idx := range state.CategoryIndices["Titles"] {
		entry := state.CosmeticList.CosmeticEntries[idx]
		dName := strings.ToLower(string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00")))
		if query == "" || strings.Contains(dName, query) {
			state.CategoryFiltered["Titles"] = append(state.CategoryFiltered["Titles"], idx)
		}
	}
	if titleList != nil {
		titleList.Refresh()
	}
}
