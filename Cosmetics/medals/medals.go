package medals

import (
	"bytes"
	"evrCosmeticResearch/Data"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// CMedal holds the editable fields for a medal cosmetic entry.
type CMedal struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64
	TextureSymbol   int64
}

// ToCosmeticEntry converts a CMedal to a raw CosmeticEntry for serialization.
func (c *CMedal) ToCosmeticEntry() (data.CosmeticEntry, error) {
	foo := data.CosmeticEntry{}
	foo.CEntry = data.NewCDescriptor()
	foo.CEntry.CosmeticTypeSymbol = int64(data.ToSymbol("medal"))
	foo.CEntry.InternalNameSymbol = int64(data.ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.CEntry.InternalNameSymbol2 = foo.CEntry.InternalNameSymbol
	copy(foo.CEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.CEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.CEntry.DescriptionString[:], []byte(c.Description))
	foo.CEntry.RaritySymbol = c.Rarity
	foo.CEntry.ThumbnailSymbol = c.ThumbnailSymbol
	foo.CEntry.TextureSymbol = c.TextureSymbol
	return foo, nil
}

// FromCosmeticEntry populates a CMedal from a raw CosmeticEntry.
func (c *CMedal) FromCosmeticEntry(d data.CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = d.CEntry.RaritySymbol
	c.ThumbnailSymbol = d.CEntry.ThumbnailSymbol
	c.TextureSymbol = d.CEntry.TextureSymbol
	return nil
}

var (
	medalList             *widget.List
	searchEntry           *widget.Entry
	medalPreviewImage     *canvas.Image
	medalReplacementImage *canvas.Image
	selectedMedalPngPath  string
	replaceMedalBtn       *widget.Button
	curMedalOrigPath      string
)

// SetupUI builds and returns the Fyne canvas object for the Medals tab.
func SetupUI(state *data.AppState) fyne.CanvasObject {
	searchEntry = widget.NewEntry()
	searchEntry.PlaceHolder = "Search Medals..."
	searchEntry.OnChanged = func(s string) { RefreshFilter(state, s) }
	medalList = widget.NewList(
		func() int { return len(state.CategoryFiltered["Medals"]) },
		func() fyne.CanvasObject { return widget.NewLabel("Template") },
		func(id widget.ListItemID, item fyne.CanvasObject) {
			realIdx := state.CategoryFiltered["Medals"][id]
			entry := state.CosmeticList.CosmeticEntries[realIdx]
			item.(*widget.Label).SetText(string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00")))
		},
	)
	medalList.OnSelected = func(id widget.ListItemID) { LoadToEditor(state, state.CategoryFiltered["Medals"][id]) }
	medalPreviewImage = canvas.NewImageFromResource(nil)
	medalPreviewImage.FillMode = canvas.ImageFillContain
	medalPreviewImage.SetMinSize(fyne.NewSize(0, 300))
	medalReplacementImage = canvas.NewImageFromResource(nil)
	medalReplacementImage.FillMode = canvas.ImageFillContain
	medalReplacementImage.SetMinSize(fyne.NewSize(0, 300))
	return container.NewBorder(searchEntry, nil, nil, nil, medalList)
}

// LoadToEditor populates the shared sidebar with the medal at the given CosmeticList index.
func LoadToEditor(state *data.AppState, realIdx int) {
	if state.SelectedIndex != realIdx || state.SelectedCategory != "Medals" {
		state.CurrentReplacementPath = ""
	}
	state.SelectedIndex = realIdx
	state.SelectedCategory = "Medals"
	state.RefreshCurrent = func(s *data.AppState) { LoadToEditor(s, realIdx) }
	t := CMedal{}
	t.FromCosmeticEntry(state.CosmeticList.CosmeticEntries[realIdx])
	state.IsLoadingEntry = true
	state.NameEntry.SetText(t.DisplayName)
	state.DescEntry.SetText(t.Description)
	state.ThumbIdEntry.SetText(data.SymbolToHex(t.ThumbnailSymbol))
	state.RaritySelect.SetSelected(state.GetRarityName(t.Rarity))
	state.UpdateSidebarThumbnail(t.ThumbnailSymbol)
	state.CurrentAssetSymbol = data.SymbolToHex(t.TextureSymbol)
	state.CurrentOriginalAssetPath = ""
	if t.TextureSymbol != 0 {
		p := filepath.Join(state.Settings.TextureCachePath, data.SymbolToHex(t.TextureSymbol)+".png")
		if _, err := os.Stat(p); err == nil {
			state.CurrentOriginalAssetPath = p
		}
	}
	texEnt := widget.NewEntry()
	texEnt.SetText(data.SymbolToHex(t.TextureSymbol))
	texEnt.OnChanged = func(s string) {
		if state.IsLoadingEntry {
			return
		}
		state.CosmeticList.CosmeticEntries[state.SelectedIndex].CEntry.TextureSymbol = data.HexToSymbol(s)
		state.CurrentAssetSymbol = s
	}
	replaceMedalBtn = widget.NewButton("Replace Medal Texture", func() {
		data.HandleTextureReplacement(state, data.SymbolToHex(t.TextureSymbol), state.CurrentReplacementPath, replaceMedalBtn, "Replacing Medal...")
	})
	if state.CurrentReplacementPath == "" {
		replaceMedalBtn.Disable()
	}

	selectMedalPngBtn := widget.NewButton("Select Replacement PNG", func() {
		path, err := data.PickFile("PNG Files (*.png)|*.png|All Files (*.*)|*.*")
		if err == nil && path != "" {
			state.CurrentReplacementPath = path
			LoadToEditor(state, realIdx)
		}
	})
	state.CategoryEditor.Objects = []fyne.CanvasObject{
		widget.NewForm(widget.NewFormItem("Texture Symbol", texEnt)),
		widget.NewLabel("Medal Texture Replacement"),
		selectMedalPngBtn,
		container.NewGridWithColumns(2,
			container.NewBorder(widget.NewLabel("Original"), nil, nil, nil, medalPreviewImage),
			container.NewBorder(widget.NewLabel("Replacement"), nil, nil, nil, medalReplacementImage),
		),
		replaceMedalBtn,
	}
	state.CategoryEditor.Refresh()
	data.RefreshAssetPreview(state, medalPreviewImage, medalReplacementImage, state.CurrentOriginalAssetPath, state.CurrentReplacementPath, true)
	state.IsLoadingEntry = false
}

// RefreshFilter re-filters the medals list to entries whose display name contains query.
func RefreshFilter(state *data.AppState, query string) {
	query = strings.ToLower(query)
	state.CategoryFiltered["Medals"] = []int{}
	for _, idx := range state.CategoryIndices["Medals"] {
		dName := strings.ToLower(string(bytes.TrimRight(state.CosmeticList.CosmeticEntries[idx].CEntry.DisplayNameString[:], "\x00")))
		if query == "" || strings.Contains(dName, query) {
			state.CategoryFiltered["Medals"] = append(state.CategoryFiltered["Medals"], idx)
		}
	}
	if medalList != nil {
		medalList.Refresh()
	}
}
