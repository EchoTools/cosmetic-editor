package patterns

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

type CPattern struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64
	TextureSymbol   int64
}

func (c *CPattern) ToCosmeticEntry() (data.CosmeticEntry, error) {
	foo := data.CosmeticEntry{}
	foo.CEntry = data.NewCDescriptor()
	foo.CEntry.CosmeticTypeSymbol = int64(data.ToSymbol("pattern"))
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

func (c *CPattern) FromCosmeticEntry(d data.CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = d.CEntry.RaritySymbol
	c.ThumbnailSymbol = d.CEntry.ThumbnailSymbol
	c.TextureSymbol = d.CEntry.TextureSymbol
	return nil
}

var (
	patternList             *widget.List
	searchEntry            *widget.Entry
	patternPreviewImage     *canvas.Image
	patternReplacementImage *canvas.Image
	selectedPatternPngPath  string
	replacePatternBtn       *widget.Button
	curPatternOrigPath      string
)

func SetupUI(state *data.AppState) fyne.CanvasObject {
	searchEntry = widget.NewEntry()
	searchEntry.PlaceHolder = "Search Patterns..."
	searchEntry.OnChanged = func(s string) { RefreshFilter(state, s) }
	patternList = widget.NewList(
		func() int { return len(state.CategoryFiltered["Patterns"]) },
		func() fyne.CanvasObject { return widget.NewLabel("Template") },
		func(id widget.ListItemID, item fyne.CanvasObject) {
			realIdx := state.CategoryFiltered["Patterns"][id]
			entry := state.CosmeticList.CosmeticEntries[realIdx]
			item.(*widget.Label).SetText(string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00")))
		},
	)
	patternList.OnSelected = func(id widget.ListItemID) { LoadToEditor(state, state.CategoryFiltered["Patterns"][id]) }
	patternPreviewImage = canvas.NewImageFromResource(nil); patternPreviewImage.FillMode = canvas.ImageFillContain; patternPreviewImage.SetMinSize(fyne.NewSize(0, 300))
	patternReplacementImage = canvas.NewImageFromResource(nil); patternReplacementImage.FillMode = canvas.ImageFillContain; patternReplacementImage.SetMinSize(fyne.NewSize(0, 300))
	return container.NewBorder(searchEntry, nil, nil, nil, patternList)
}

func LoadToEditor(state *data.AppState, realIdx int) {
	if state.SelectedIndex != realIdx || state.SelectedCategory != "Patterns" {
		state.CurrentReplacementPath = ""
	}
	state.SelectedIndex = realIdx; state.SelectedCategory = "Patterns"
	state.RefreshCurrent = func(s *data.AppState) { LoadToEditor(s, realIdx) }
	t := CPattern{}; t.FromCosmeticEntry(state.CosmeticList.CosmeticEntries[realIdx])
	state.IsLoadingEntry = true
	state.NameEntry.SetText(t.DisplayName); state.DescEntry.SetText(t.Description)
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
	texEnt := widget.NewEntry(); texEnt.SetText(data.SymbolToHex(t.TextureSymbol))
	texEnt.OnChanged = func(s string) {
		if state.IsLoadingEntry { return }
		state.CosmeticList.CosmeticEntries[state.SelectedIndex].CEntry.TextureSymbol = data.HexToSymbol(s)
		state.CurrentAssetSymbol = s
	}
	replacePatternBtn = widget.NewButton("Replace Pattern Texture", func() {
		data.HandleTextureReplacement(state, data.SymbolToHex(t.TextureSymbol), state.CurrentReplacementPath, replacePatternBtn, "Replacing Pattern...")
	})
	if state.CurrentReplacementPath == "" {
		replacePatternBtn.Disable()
	}

	selectPatternPngBtn := widget.NewButton("Select Replacement PNG", func() {
		path, err := data.PickFile("PNG Files (*.png)|*.png|All Files (*.*)|*.*")
		if err == nil && path != "" {
			state.CurrentReplacementPath = path
			LoadToEditor(state, realIdx)
		}
	})
	state.CategoryEditor.Objects = []fyne.CanvasObject{
		widget.NewForm(widget.NewFormItem("Texture Symbol", texEnt)),
		widget.NewLabel("Pattern Texture Replacement"),
		selectPatternPngBtn,
		container.NewGridWithColumns(2,
			container.NewBorder(widget.NewLabel("Original"), nil, nil, nil, patternPreviewImage),
			container.NewBorder(widget.NewLabel("Replacement"), nil, nil, nil, patternReplacementImage),
		),
		replacePatternBtn,
	}
	state.CategoryEditor.Refresh()
	data.RefreshAssetPreview(state, patternPreviewImage, patternReplacementImage, state.CurrentOriginalAssetPath, state.CurrentReplacementPath, true)
	state.IsLoadingEntry = false
}

func RefreshFilter(state *data.AppState, query string) {
	query = strings.ToLower(query); state.CategoryFiltered["Patterns"] = []int{}
	for _, idx := range state.CategoryIndices["Patterns"] {
		dName := strings.ToLower(string(bytes.TrimRight(state.CosmeticList.CosmeticEntries[idx].CEntry.DisplayNameString[:], "\x00")))
		if query == "" || strings.Contains(dName, query) { state.CategoryFiltered["Patterns"] = append(state.CategoryFiltered["Patterns"], idx) }
	}
	if patternList != nil { patternList.Refresh() }
}
