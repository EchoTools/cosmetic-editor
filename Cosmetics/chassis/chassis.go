package chassis

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
	chassisList *widget.List
	searchEntry *widget.Entry
)

func SetupUI(state *data.AppState) fyne.CanvasObject {
	searchEntry = widget.NewEntry()
	searchEntry.PlaceHolder = "Search Chassis..."
	searchEntry.OnChanged = func(s string) {
		RefreshFilter(state, s)
	}

	chassisList = widget.NewList(
		func() int {
			return len(state.CategoryFiltered["Chassis"])
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			realIdx := state.CategoryFiltered["Chassis"][id]
			entry := state.CosmeticList.CosmeticEntries[realIdx]
			dName := string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00"))
			item.(*widget.Label).SetText(dName)
		},
	)

	chassisList.OnSelected = func(id widget.ListItemID) {
		realIdx := state.CategoryFiltered["Chassis"][id]
		LoadToEditor(state, realIdx)
	}

	content := container.NewBorder(
		container.NewVBox(searchEntry),
		nil, nil, nil, chassisList,
	)

	return content
}

func LoadToEditor(state *data.AppState, realIdx int) {
	state.SelectedIndex = realIdx
	state.SelectedCategory = "Chassis"
	state.RefreshCurrent = func(s *data.AppState) { LoadToEditor(s, realIdx) }

	c := data.CChassis{}
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

	sym6Entry := widget.NewEntry()
	sym6Str := fmt.Sprintf("%016x", uint64(c.AssetSymbol6))
	if len(sym6Str) == 16 {
		sym6Entry.SetText(sym6Str[1:])
	} else {
		sym6Entry.SetText(sym6Str)
	}
	sym12Entry := widget.NewEntry()
	sym12Entry.SetText(fmt.Sprintf("%016x", uint64(c.AssetSymbol12)))

	saveSymbols := func() {
		if state.IsLoadingEntry {
			return
		}
		entry := &state.CosmeticList.CosmeticEntries[state.SelectedIndex]

		entry.CEntry.AssetSymbol6 = data.HexToSymbol(sym6Entry.Text)
		entry.CEntry.AssetSymbol12 = data.HexToSymbol(sym12Entry.Text)
		state.AutoSave()
	}

	sym6Entry.OnChanged = func(string) { saveSymbols() }
	sym12Entry.OnChanged = func(string) { saveSymbols() }

	state.CategoryEditor.Objects = []fyne.CanvasObject{
		widget.NewForm(
			widget.NewFormItem("1st Person Hash (Sym6)", sym6Entry),
			widget.NewFormItem("3rd Person Hash (Sym12)", sym12Entry),
		),
	}
	state.CategoryEditor.Refresh()

	state.IsLoadingEntry = false
}

func RefreshFilter(state *data.AppState, query string) {
	query = strings.ToLower(query)
	state.CategoryFiltered["Chassis"] = []int{}
	for _, idx := range state.CategoryIndices["Chassis"] {
		entry := state.CosmeticList.CosmeticEntries[idx]
		dName := strings.ToLower(string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00")))
		if query == "" || strings.Contains(dName, query) {
			state.CategoryFiltered["Chassis"] = append(state.CategoryFiltered["Chassis"], idx)
		}
	}
	if chassisList != nil {
		chassisList.Refresh()
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
