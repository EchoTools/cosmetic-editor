package emblems

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
	emblemList             *widget.List
	searchEntry            *widget.Entry
	emblemPreviewImage     *canvas.Image
	emblemReplacementImage *canvas.Image
	replaceEmblemBtn       *widget.Button
)

// SetupUI builds and returns the Fyne canvas object for the Emblems tab.
func SetupUI(state *data.AppState) fyne.CanvasObject {
	searchEntry = widget.NewEntry()
	searchEntry.PlaceHolder = "Search Emblems..."
	searchEntry.OnChanged = func(s string) { RefreshFilter(state, s) }
	emblemList = widget.NewList(
		func() int { return len(state.CategoryFiltered["Emblems"]) },
		func() fyne.CanvasObject { return widget.NewLabel("Template") },
		func(id widget.ListItemID, item fyne.CanvasObject) {
			realIdx := state.CategoryFiltered["Emblems"][id]
			entry := state.CosmeticList.CosmeticEntries[realIdx]
			item.(*widget.Label).SetText(string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00")))
		},
	)
	emblemList.OnSelected = func(id widget.ListItemID) { LoadToEditor(state, state.CategoryFiltered["Emblems"][id]) }
	emblemPreviewImage = canvas.NewImageFromResource(nil)
	emblemPreviewImage.FillMode = canvas.ImageFillContain
	emblemPreviewImage.SetMinSize(fyne.NewSize(0, 300))
	emblemReplacementImage = canvas.NewImageFromResource(nil)
	emblemReplacementImage.FillMode = canvas.ImageFillContain
	emblemReplacementImage.SetMinSize(fyne.NewSize(0, 300))
	return container.NewBorder(searchEntry, nil, nil, nil, emblemList)
}

// LoadToEditor populates the shared sidebar with the emblem at the given CosmeticList index.
func LoadToEditor(state *data.AppState, realIdx int) {
	if state.SelectedIndex != realIdx || state.SelectedCategory != "Emblems" {
		state.CurrentReplacementPath = ""
	}
	state.SelectedIndex = realIdx
	state.SelectedCategory = "Emblems"
	state.RefreshCurrent = func(s *data.AppState) { LoadToEditor(s, realIdx) }

	entry := state.CosmeticList.CosmeticEntries[realIdx]
	e := data.CEmblem{}
	e.FromCosmeticEntry(entry)

	state.IsLoadingEntry = true
	state.NameEntry.SetText(e.DisplayName)
	state.DescEntry.SetText(e.Description)
	state.ThumbIdEntry.SetText(data.SymbolToHex(data.HexToSymbol(e.ThumbnailSymbol)))
	state.RaritySelect.SetSelected(state.GetRarityName(data.HexToSymbol(e.Rarity)))
	state.UpdateSidebarThumbnail(data.HexToSymbol(e.ThumbnailSymbol))
	state.CurrentAssetSymbol = e.TextureSymbol
	state.CurrentOriginalAssetPath = ""
	if e.TextureSymbol != "" {
		p := filepath.Join(state.Settings.TextureCachePath, e.TextureSymbol+".png")
		if _, err := os.Stat(p); err == nil {
			state.CurrentOriginalAssetPath = p
		}
	}
	texEnt := widget.NewEntry()
	texEnt.SetText(e.TextureSymbol)
	texEnt.OnChanged = func(s string) {
		if state.IsLoadingEntry {
			return
		}
		state.CosmeticList.CosmeticEntries[state.SelectedIndex].CEntry.TextureSymbol = data.HexToSymbol(s)
		state.CurrentAssetSymbol = s
	}
	replaceEmblemBtn = widget.NewButton("Replace Emblem Texture", func() {
		data.HandleTextureReplacement(state, e.TextureSymbol, state.CurrentReplacementPath, replaceEmblemBtn, "Replacing Emblem...")
	})
	if state.CurrentReplacementPath == "" {
		replaceEmblemBtn.Disable()
	}

	selectEmblemPngBtn := widget.NewButton("Select Replacement PNG", func() {
		path, err := data.PickFile("PNG Files (*.png)|*.png|All Files (*.*)|*.*")
		if err == nil && path != "" {
			state.CurrentReplacementPath = path
			LoadToEditor(state, realIdx)
		}
	})
	state.CategoryEditor.Objects = []fyne.CanvasObject{
		widget.NewForm(widget.NewFormItem("Texture Symbol", texEnt)),
		widget.NewLabel("Emblem Texture Replacement"),
		selectEmblemPngBtn,
		container.NewGridWithColumns(2,
			container.NewBorder(widget.NewLabel("Original"), nil, nil, nil, emblemPreviewImage),
			container.NewBorder(widget.NewLabel("Replacement"), nil, nil, nil, emblemReplacementImage),
		),
		replaceEmblemBtn,
	}
	state.CategoryEditor.Refresh()
	data.RefreshAssetPreview(state, emblemPreviewImage, emblemReplacementImage, state.CurrentOriginalAssetPath, state.CurrentReplacementPath, true)
	state.IsLoadingEntry = false
}

// RefreshFilter re-filters the emblem list to entries whose display name contains query.
func RefreshFilter(state *data.AppState, query string) {
	query = strings.ToLower(query)
	state.CategoryFiltered["Emblems"] = []int{}
	for _, idx := range state.CategoryIndices["Emblems"] {
		dName := strings.ToLower(string(bytes.TrimRight(state.CosmeticList.CosmeticEntries[idx].CEntry.DisplayNameString[:], "\x00")))
		if query == "" || strings.Contains(dName, query) {
			state.CategoryFiltered["Emblems"] = append(state.CategoryFiltered["Emblems"], idx)
		}
	}
	if emblemList != nil {
		emblemList.Refresh()
	}
}
