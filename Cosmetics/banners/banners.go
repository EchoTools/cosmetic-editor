package banners

import (
	"bytes"
	"evrCosmeticResearch/Data"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// CBanner holds the editable fields for a banner cosmetic entry.
type CBanner struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64
	TextureSymbol   int64

	MedalXPos    float32
	MedalYPos    float32
	MedalHeight  float32
	MedalWidth   float32
	EmblemXPos   float32
	EmblemYPos   float32
	EmblemHeight float32
	EmblemWidth  float32
}

// ToCosmeticEntry converts a CBanner to a raw CosmeticEntry for serialization.
func (c *CBanner) ToCosmeticEntry() (data.CosmeticEntry, error) {
	foo := data.CosmeticEntry{}
	foo.CEntry = data.NewCDescriptor()

	foo.CEntry.CosmeticTypeSymbol = int64(data.ToSymbol("banner"))
	foo.CEntry.InternalNameSymbol = int64(data.ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.CEntry.InternalNameSymbol2 = foo.CEntry.InternalNameSymbol

	copy(foo.CEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.CEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.CEntry.DescriptionString[:], []byte(c.Description))

	foo.CEntry.RaritySymbol = c.Rarity
	foo.CEntry.ThumbnailSymbol = c.ThumbnailSymbol
	foo.CEntry.TextureSymbol = c.TextureSymbol

	foo.CEntry.BannerMedalXPos = c.MedalXPos
	foo.CEntry.BannerMedalYPos = c.MedalYPos
	foo.CEntry.BannerMedalHeight = c.MedalHeight
	foo.CEntry.BannerMedalWidth = c.MedalWidth
	foo.CEntry.BannerEmblemXPos = c.EmblemXPos
	foo.CEntry.BannerEmblemYPos = c.EmblemYPos
	foo.CEntry.BannerEmblemHeight = c.EmblemHeight
	foo.CEntry.BannerEmblemWidth = c.EmblemWidth

	return foo, nil
}

// FromCosmeticEntry populates a CBanner from a raw CosmeticEntry.
func (c *CBanner) FromCosmeticEntry(d data.CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = d.CEntry.RaritySymbol
	c.ThumbnailSymbol = d.CEntry.ThumbnailSymbol
	c.TextureSymbol = d.CEntry.TextureSymbol

	c.MedalXPos = d.CEntry.BannerMedalXPos
	c.MedalYPos = d.CEntry.BannerMedalYPos
	c.MedalHeight = d.CEntry.BannerMedalHeight
	c.MedalWidth = d.CEntry.BannerMedalWidth
	c.EmblemXPos = d.CEntry.BannerEmblemXPos
	c.EmblemYPos = d.CEntry.BannerEmblemYPos
	c.EmblemHeight = d.CEntry.BannerEmblemHeight
	c.EmblemWidth = d.CEntry.BannerEmblemWidth
	return nil
}

var (
	bannerList  *widget.List
	searchEntry *widget.Entry

	// Local Preview Widgets
	bannerPreviewImage     *canvas.Image
	bannerReplacementImage *canvas.Image
	selectedBannerPngPath  string
	replaceBannerBtn       *widget.Button
	curBannerOrigPath      string
	curBannerReplPath      string
)

// SetupUI builds and returns the Fyne canvas object for the Banners tab.
func SetupUI(state *data.AppState) fyne.CanvasObject {
	searchEntry = widget.NewEntry()
	searchEntry.PlaceHolder = "Search Banners..."
	searchEntry.OnChanged = func(s string) {
		RefreshFilter(state, s)
	}

	bannerList = widget.NewList(
		func() int {
			return len(state.CategoryFiltered["Banners"])
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			realIdx := state.CategoryFiltered["Banners"][id]
			entry := state.CosmeticList.CosmeticEntries[realIdx]
			dName := string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00"))
			item.(*widget.Label).SetText(dName)
		},
	)

	bannerList.OnSelected = func(id widget.ListItemID) {
		realIdx := state.CategoryFiltered["Banners"][id]
		LoadToEditor(state, realIdx)
	}

	// Initialize Local Previews
	bannerPreviewImage = canvas.NewImageFromResource(nil)
	bannerPreviewImage.FillMode = canvas.ImageFillContain
	bannerPreviewImage.SetMinSize(fyne.NewSize(0, 200))

	bannerReplacementImage = canvas.NewImageFromResource(nil)
	bannerReplacementImage.FillMode = canvas.ImageFillContain
	bannerReplacementImage.SetMinSize(fyne.NewSize(0, 200))

	content := container.NewBorder(searchEntry, nil, nil, nil, bannerList)

	return content
}

// LoadToEditor populates the shared sidebar with the banner at the given CosmeticList index.
func LoadToEditor(state *data.AppState, realIdx int) {
	if state.SelectedIndex != realIdx || state.SelectedCategory != "Banners" {
		state.CurrentReplacementPath = ""
	}
	state.SelectedIndex = realIdx
	state.SelectedCategory = "Banners"
	state.RefreshCurrent = func(s *data.AppState) { LoadToEditor(s, realIdx) }

	t := CBanner{}
	if err := t.FromCosmeticEntry(state.CosmeticList.CosmeticEntries[realIdx]); err != nil {
		return
	}

	state.IsLoadingEntry = true
	state.NameEntry.SetText(t.DisplayName)
	state.DescEntry.SetText(t.Description)
	state.ThumbIdEntry.SetText(data.SymbolToHex(t.ThumbnailSymbol))
	state.RaritySelect.SetSelected(state.GetRarityName(t.Rarity))
	state.UpdateSidebarThumbnail(t.ThumbnailSymbol)

	state.CurrentAssetSymbol = data.SymbolToHex(t.TextureSymbol)

	// Track paths for replacement grid
	curBannerOrigPath = ""
	if t.TextureSymbol != 0 {
		hexStr := data.SymbolToHex(t.TextureSymbol)
		p := filepath.Join(state.Settings.TextureCachePath, hexStr+".png")
		if _, err := os.Stat(p); err == nil {
			curBannerOrigPath = p
		}
	}

	// POSITIONING UI
	mx := widget.NewEntry()
	mx.SetText(fmt.Sprintf("%.2f", t.MedalXPos))
	my := widget.NewEntry()
	my.SetText(fmt.Sprintf("%.2f", t.MedalYPos))
	mh := widget.NewEntry()
	mh.SetText(fmt.Sprintf("%.2f", t.MedalHeight))
	mw := widget.NewEntry()
	mw.SetText(fmt.Sprintf("%.2f", t.MedalWidth))

	ex := widget.NewEntry()
	ex.SetText(fmt.Sprintf("%.2f", t.EmblemXPos))
	ey := widget.NewEntry()
	ey.SetText(fmt.Sprintf("%.2f", t.EmblemYPos))
	eh := widget.NewEntry()
	eh.SetText(fmt.Sprintf("%.2f", t.EmblemHeight))
	ew := widget.NewEntry()
	ew.SetText(fmt.Sprintf("%.2f", t.EmblemWidth))

	texEnt := widget.NewEntry()
	texEnt.SetText(data.SymbolToHex(t.TextureSymbol))

	saveChanges := func() {
		if state.IsLoadingEntry {
			return
		}
		entry := &state.CosmeticList.CosmeticEntries[state.SelectedIndex]

		entry.CEntry.TextureSymbol = data.HexToSymbol(texEnt.Text)
		f, _ := strconv.ParseFloat(mx.Text, 32)
		entry.CEntry.BannerMedalXPos = float32(f)
		f, _ = strconv.ParseFloat(my.Text, 32)
		entry.CEntry.BannerMedalYPos = float32(f)
		f, _ = strconv.ParseFloat(mh.Text, 32)
		entry.CEntry.BannerMedalHeight = float32(f)
		f, _ = strconv.ParseFloat(mw.Text, 32)
		entry.CEntry.BannerMedalWidth = float32(f)
		f, _ = strconv.ParseFloat(ex.Text, 32)
		entry.CEntry.BannerEmblemXPos = float32(f)
		f, _ = strconv.ParseFloat(ey.Text, 32)
		entry.CEntry.BannerEmblemYPos = float32(f)
		f, _ = strconv.ParseFloat(eh.Text, 32)
		entry.CEntry.BannerEmblemHeight = float32(f)
		f, _ = strconv.ParseFloat(ew.Text, 32)
		entry.CEntry.BannerEmblemWidth = float32(f)
	}

	mx.OnChanged = func(string) { saveChanges() }
	my.OnChanged = func(string) { saveChanges() }
	mh.OnChanged = func(string) { saveChanges() }
	mw.OnChanged = func(string) { saveChanges() }
	ex.OnChanged = func(string) { saveChanges() }
	ey.OnChanged = func(string) { saveChanges() }
	eh.OnChanged = func(string) { saveChanges() }
	ew.OnChanged = func(string) { saveChanges() }
	texEnt.OnChanged = func(s string) {
		saveChanges()
		state.CurrentAssetSymbol = s
	}

	// REPLACEMENT UI
	replaceBannerBtn = widget.NewButton("Replace Banner Texture", func() {
		data.HandleTextureReplacement(state, data.SymbolToHex(t.TextureSymbol), state.CurrentReplacementPath, replaceBannerBtn, "Replacing Banner...")
	})
	if state.CurrentReplacementPath == "" {
		replaceBannerBtn.Disable()
	}

	selectBannerPngBtn := widget.NewButton("Select Replacement PNG", func() {
		path, err := data.PickFile("PNG Files (*.png)|*.png|All Files (*.*)|*.*")
		if err == nil && path != "" {
			state.CurrentReplacementPath = path
			LoadToEditor(state, realIdx) // Refresh to show replacement with potential tint
		}
	})

	// RECONSTRUCT EDITOR UI FOR PARITY
	state.CategoryEditor.Objects = []fyne.CanvasObject{
		widget.NewForm(widget.NewFormItem("Texture Symbol", texEnt)),
		widget.NewLabel("Banner Texture Replacement"),
		selectBannerPngBtn,
		container.NewGridWithColumns(2,
			container.NewBorder(widget.NewLabel("Original"), nil, nil, nil, bannerPreviewImage),
			container.NewBorder(widget.NewLabel("Replacement"), nil, nil, nil, bannerReplacementImage),
		),
		replaceBannerBtn,
		widget.NewSeparator(),
		widget.NewLabel("Medal Position"),
		widget.NewForm(
			widget.NewFormItem("X Pos", mx),
			widget.NewFormItem("Y Pos", my),
			widget.NewFormItem("Height", mh),
			widget.NewFormItem("Width", mw),
		),
		widget.NewSeparator(),
		widget.NewLabel("Emblem Position"),
		widget.NewForm(
			widget.NewFormItem("X Pos", ex),
			widget.NewFormItem("Y Pos", ey),
			widget.NewFormItem("Height", eh),
			widget.NewFormItem("Width", ew),
		),
	}
	state.CategoryEditor.Refresh()

	// Initial Preview Update
	data.RefreshAssetPreview(state, bannerPreviewImage, bannerReplacementImage, curBannerOrigPath, "", true)

	state.IsLoadingEntry = false
}

// RefreshFilter re-filters the banners list to entries whose display name contains query.
func RefreshFilter(state *data.AppState, query string) {
	query = strings.ToLower(query)
	state.CategoryFiltered["Banners"] = []int{}
	for _, idx := range state.CategoryIndices["Banners"] {
		entry := state.CosmeticList.CosmeticEntries[idx]
		dName := strings.ToLower(string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00")))
		if query == "" || strings.Contains(dName, query) {
			state.CategoryFiltered["Banners"] = append(state.CategoryFiltered["Banners"], idx)
		}
	}
	if bannerList != nil {
		bannerList.Refresh()
	}
}
