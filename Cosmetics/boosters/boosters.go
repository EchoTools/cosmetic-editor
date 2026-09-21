package boosters

import (
	"bytes"
	"fmt"
	data "github.com/EchoTools/cosmetic-editor/Data"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var (
	boosterList *widget.List
	searchEntry *widget.Entry
)

func SetupUI(state *data.AppState) fyne.CanvasObject {
	searchEntry = widget.NewEntry()
	searchEntry.PlaceHolder = "Search Boosters..."
	searchEntry.OnChanged = func(s string) {
		RefreshFilter(state, s)
	}

	boosterList = widget.NewList(
		func() int {
			return len(state.CategoryFiltered["Boosters"])
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			realIdx := state.CategoryFiltered["Boosters"][id]
			entry := state.CosmeticList.CosmeticEntries[realIdx]
			dName := string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00"))
			item.(*widget.Label).SetText(dName)
		},
	)

	boosterList.OnSelected = func(id widget.ListItemID) {
		realIdx := state.CategoryFiltered["Boosters"][id]
		LoadToEditor(state, realIdx)
	}

	content := container.NewBorder(
		container.NewVBox(searchEntry),
		nil, nil, nil, boosterList,
	)

	return content
}

func LoadToEditor(state *data.AppState, realIdx int) {
	state.SelectedIndex = realIdx
	state.SelectedCategory = "Boosters"
	state.RefreshCurrent = func(s *data.AppState) { LoadToEditor(s, realIdx) }

	c := data.CBooster{}
	if err := c.FromCosmeticEntry(state.CosmeticList.CosmeticEntries[realIdx]); err != nil {
		return
	}

	state.IsLoadingEntry = true
	state.NameEntry.SetText(c.DisplayName)
	state.DescEntry.SetText(c.Description)
	state.ThumbIdEntry.SetText(c.ThumbnailSymbol)
	state.RaritySelect.SetSelected(state.GetRarityName(data.HexToSymbol(c.Rarity)))
	state.UpdateSidebarThumbnail(data.HexToSymbol(c.ThumbnailSymbol))

	state.CurrentAssetSymbol = c.TextureSymbol

	sym12Entry := widget.NewEntry()
	sym12Entry.SetText(fmt.Sprintf("%016x", uint64(c.AssetSymbol12)))

	saveSymbols := func() {
		if state.IsLoadingEntry {
			return
		}
		entry := &state.CosmeticList.CosmeticEntries[state.SelectedIndex]

		entry.CEntry.AssetSymbol12 = data.HexToSymbol(sym12Entry.Text)
		state.AutoSave()
	}

	sym12Entry.OnChanged = func(string) { saveSymbols() }

	state.CategoryEditor.Objects = []fyne.CanvasObject{
		widget.NewForm(
			widget.NewFormItem("Mesh Hash (Sym12)", sym12Entry),
		),
	}
	state.CategoryEditor.Refresh()

	state.IsLoadingEntry = false
}

func RefreshFilter(state *data.AppState, query string) {
	query = strings.ToLower(query)
	state.CategoryFiltered["Boosters"] = []int{}
	for _, idx := range state.CategoryIndices["Boosters"] {
		entry := state.CosmeticList.CosmeticEntries[idx]
		dName := strings.ToLower(string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00")))
		if query == "" || strings.Contains(dName, query) {
			state.CategoryFiltered["Boosters"] = append(state.CategoryFiltered["Boosters"], idx)
		}
	}
	if boosterList != nil {
		boosterList.Refresh()
	}
}

type Manifest struct {
	AssetSymbol5   uint64 `json:"AssetSymbol5"`
	AssetSymbol6   uint64 `json:"AssetSymbol6"`
	AssetSymbol11  uint64 `json:"AssetSymbol11"`
	AssetSymbol12  uint64 `json:"AssetSymbol12"`
	MeshHashHex    string `json:"MeshHashHex"`
	RigHashHex     string `json:"RigHashHex"`
	TextureHashHex string `json:"TextureHashHex"`
	MeshFileGpu    string `json:"MeshFileGpu"`
	MeshFilePri    string `json:"MeshFilePri"`
	RigFileGpu     string `json:"RigFileGpu"`
	RigFilePri     string `json:"RigFilePri"`
	TextureFile    string `json:"TextureFile"`
}
