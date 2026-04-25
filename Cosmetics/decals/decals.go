package decals

import (
	"bytes"
	"github.com/EchoTools/cosmetic-editor/Data"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var (
	decalList             *widget.List
	searchEntry           *widget.Entry
	decalPreviewImage     *canvas.Image
	decalReplacementImage *canvas.Image
	replaceDecalBtn       *widget.Button
)

func SetupUI(state *data.AppState) fyne.CanvasObject {
	searchEntry = widget.NewEntry()
	searchEntry.PlaceHolder = "Search Decals..."
	searchEntry.OnChanged = func(s string) { RefreshFilter(state, s) }
	decalList = widget.NewList(
		func() int { return len(state.CategoryFiltered["Decals"]) },
		func() fyne.CanvasObject { return widget.NewLabel("Template") },
		func(id widget.ListItemID, item fyne.CanvasObject) {
			realIdx := state.CategoryFiltered["Decals"][id]
			entry := state.CosmeticList.CosmeticEntries[realIdx]
			item.(*widget.Label).SetText(string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00")))
		},
	)
	decalList.OnSelected = func(id widget.ListItemID) { LoadToEditor(state, state.CategoryFiltered["Decals"][id]) }
	decalPreviewImage = canvas.NewImageFromResource(nil); decalPreviewImage.FillMode = canvas.ImageFillContain; decalPreviewImage.SetMinSize(fyne.NewSize(0, 300))
	decalReplacementImage = canvas.NewImageFromResource(nil); decalReplacementImage.FillMode = canvas.ImageFillContain; decalReplacementImage.SetMinSize(fyne.NewSize(0, 300))
	return container.NewBorder(searchEntry, nil, nil, nil, decalList)
}

func LoadToEditor(state *data.AppState, realIdx int) {
	if state.SelectedIndex != realIdx || state.SelectedCategory != "Decals" {
		state.CurrentReplacementPath = ""
	}
	state.SelectedIndex = realIdx; state.SelectedCategory = "Decals"
	state.RefreshCurrent = func(s *data.AppState) { LoadToEditor(s, realIdx) }
	
	entry := state.CosmeticList.CosmeticEntries[realIdx]
	d := data.CDecal{}
	d.FromCosmeticEntry(entry)

	state.IsLoadingEntry = true
	state.NameEntry.SetText(d.DisplayName); state.DescEntry.SetText(d.Description)
	state.ThumbIdEntry.SetText(data.SymbolToHex(data.HexToSymbol(d.ThumbnailSymbol)))
	state.RaritySelect.SetSelected(state.GetRarityName(data.HexToSymbol(d.Rarity)))
	state.UpdateSidebarThumbnail(data.HexToSymbol(d.ThumbnailSymbol))
	state.CurrentAssetSymbol = d.TextureSymbol
	state.CurrentOriginalAssetPath = ""
	if d.TextureSymbol != "" {
		p := filepath.Join(state.Settings.TextureCachePath, d.TextureSymbol+".png")
		if _, err := os.Stat(p); err == nil {
			state.CurrentOriginalAssetPath = p
		}
	}
	texEnt := widget.NewEntry(); texEnt.SetText(d.TextureSymbol)
	texEnt.OnChanged = func(s string) {
		if state.IsLoadingEntry { return }
		state.CosmeticList.CosmeticEntries[state.SelectedIndex].CEntry.TextureSymbol = data.HexToSymbol(s)
		state.CurrentAssetSymbol = s
	}
	replaceDecalBtn = widget.NewButton("Replace Decal Texture", func() {
		data.HandleTextureReplacement(state, d.TextureSymbol, state.CurrentReplacementPath, replaceDecalBtn, "Replacing Decal...")
	})
	if state.CurrentReplacementPath == "" {
		replaceDecalBtn.Disable()
	}

	selectDecalPngBtn := widget.NewButton("Select Replacement PNG", func() {
		path, err := data.PickFile("PNG Files (*.png)|*.png|All Files (*.*)|*.*")
		if err == nil && path != "" {
			state.CurrentReplacementPath = path
			LoadToEditor(state, realIdx)
		}
	})
	state.CategoryEditor.Objects = []fyne.CanvasObject{
		widget.NewForm(widget.NewFormItem("Texture Symbol", texEnt)),
		widget.NewLabel("Decal Texture Replacement"),
		selectDecalPngBtn,
		container.NewGridWithColumns(2,
			container.NewBorder(widget.NewLabel("Original"), nil, nil, nil, decalPreviewImage),
			container.NewBorder(widget.NewLabel("Replacement"), nil, nil, nil, decalReplacementImage),
		),
		replaceDecalBtn,
	}
	state.CategoryEditor.Refresh()
	data.RefreshAssetPreview(state, decalPreviewImage, decalReplacementImage, state.CurrentOriginalAssetPath, state.CurrentReplacementPath, true)
	state.IsLoadingEntry = false
}

func RefreshFilter(state *data.AppState, query string) {
	query = strings.ToLower(query); state.CategoryFiltered["Decals"] = []int{}
	for _, idx := range state.CategoryIndices["Decals"] {
		dName := strings.ToLower(string(bytes.TrimRight(state.CosmeticList.CosmeticEntries[idx].CEntry.DisplayNameString[:], "\x00")))
		if query == "" || strings.Contains(dName, query) { state.CategoryFiltered["Decals"] = append(state.CategoryFiltered["Decals"], idx) }
	}
	if decalList != nil { decalList.Refresh() }
}
