package main

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"image/color"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"evrCosmeticResearch/Cosmetics/banners"
	"evrCosmeticResearch/Cosmetics/decals"
	"evrCosmeticResearch/Cosmetics/emblems"
	"evrCosmeticResearch/Cosmetics/emissives"
	"evrCosmeticResearch/Cosmetics/emotes"
	"evrCosmeticResearch/Cosmetics/fanfares"
	"evrCosmeticResearch/Cosmetics/medals"
	"evrCosmeticResearch/Cosmetics/patterns"
	"evrCosmeticResearch/Cosmetics/pips"
	"evrCosmeticResearch/Cosmetics/tags"
	"evrCosmeticResearch/Cosmetics/tints"
	"evrCosmeticResearch/Cosmetics/titles"
	data "evrCosmeticResearch/Data"
)

// --- EMBEDDED FILES ---
//
//go:embed Data/0x43934c379cf1e366_original
var embeddedOriginal []byte

//go:embed icon.ico
var embeddedIcon []byte

//go:embed Data/template_thumb.png
var embeddedTemplate []byte

const (
	SettingsDirName = "settings"
)

var rarityOptions = []string{"Default", "Common", "Fine", "Superb", "Epic", "Legendary", "Mythic"}

// Global App State
var state *data.AppState

// Persistent Paths
var (
	settingsFile string
	tempFilePath string
)

func main() {
	os.Setenv("FYNE_GL_VERSION", "2.1")

	// 1. SETUP: Directories & Settings
	settingsPath := data.GetSettingsDir()
	os.MkdirAll(settingsPath, 0755)

	tempDir := filepath.Join(settingsPath, "Temp")
	os.MkdirAll(tempDir, 0755)
	tempFilePath = filepath.Join(tempDir, "temp_autosave.dat")

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	if strings.Contains(strings.ToLower(exeDir), "go-build") || strings.Contains(strings.ToLower(exeDir), "temp") {
		cwd, _ := os.Getwd()
		exeDir = cwd
	}
	settingsFile = filepath.Join(exeDir, "settings.json")

	a := app.New()
	a.SetIcon(fyne.NewStaticResource("icon.ico", embeddedIcon))
	w := a.NewWindow("EchoVR Cosmetics Editor")
	w.Resize(fyne.NewSize(1200, 850))

	state = data.NewAppState(a, w)
	loadSettings()

	// 2. STABILIZE PATHS
	if state.Settings.EchoVRDataPath == "" {
		state.Settings.EchoVRDataPath = data.GetDefaultEchoVRPath()
	}
	if state.Settings.TextureCachePath == "" {
		state.Settings.TextureCachePath = filepath.Join(settingsPath, "texture_cache")
	}
	state.Settings.TextureCachePath, _ = filepath.Abs(state.Settings.TextureCachePath)

	if state.Settings.Mode == "" {
		state.Settings.Mode = "PCVR"
	}

	// Application Theme
	a.Settings().SetTheme(&data.ModernTheme{})

	data.EmbeddedTemplate = embeddedTemplate

	// UI Component Initialization
	state.StatusLabel = widget.NewLabel("Ready.")
	state.NameEntry = widget.NewEntry()
	state.DescEntry = widget.NewEntry()
	state.ThumbIdEntry = widget.NewEntry()
	state.RaritySelect = widget.NewSelect(rarityOptions, func(s string) {
		if state.SelectedIndex == -1 || state.IsLoadingEntry { return }
		entry := &state.CosmeticList.CosmeticEntries[state.SelectedIndex]
		entry.CEntry.RaritySymbol = state.GetRaritySymbol(s)
	})
	state.RaritySelect.PlaceHolder = "Select Rarity" // Optional: gives a better default than "Select One"

	state.ThumbImage = canvas.NewImageFromResource(nil)
	state.ThumbImage.FillMode = canvas.ImageFillContain
	state.ThumbImage.SetMinSize(fyne.NewSize(128, 128))

	state.TextureImage = canvas.NewImageFromResource(nil)
	state.TextureImage.FillMode = canvas.ImageFillContain
	state.TextureImage.SetMinSize(fyne.NewSize(256, 256))

	// Listeners
	state.NameEntry.OnChanged = func(s string) {
		if state.SelectedIndex == -1 || state.IsLoadingEntry { return }
		entry := &state.CosmeticList.CosmeticEntries[state.SelectedIndex]
		copy(entry.CEntry.DisplayNameString[:], []byte(s))
	}
	state.DescEntry.OnChanged = func(s string) {
		if state.SelectedIndex == -1 || state.IsLoadingEntry { return }
		entry := &state.CosmeticList.CosmeticEntries[state.SelectedIndex]
		copy(entry.CEntry.DescriptionString[:], []byte(s))
	}
	state.ThumbIdEntry.OnChanged = func(s string) {
		if state.SelectedIndex == -1 || state.IsLoadingEntry { return }
		entry := &state.CosmeticList.CosmeticEntries[state.SelectedIndex]
		entry.CEntry.ThumbnailSymbol = data.HexToSymbol(s)
	}

	state.CategoryEditor = container.NewVBox()

	// 2. NAVIGATION & CONTENT ASSEMBLY
	catNames := []string{"Tints", "Titles", "Emissives", "Fanfares", "Emotes", "Banners", "Tags", "Emblems", "Decals", "Medals", "Pips", "Patterns"}
	catIcons := []fyne.Resource{
		theme.ColorPaletteIcon(), theme.DocumentIcon(), theme.VisibilityIcon(), theme.VolumeUpIcon(), theme.MediaPlayIcon(),
		theme.GridIcon(), theme.ContentCopyIcon(), theme.ComputerIcon(), theme.CheckButtonIcon(), theme.HelpIcon(), theme.MenuIcon(), theme.ListIcon(),
	}

	catUIs := []fyne.CanvasObject{
		tints.SetupUI(state), titles.SetupUI(state), emissives.SetupUI(state), fanfares.SetupUI(state), emotes.SetupUI(state),
		banners.SetupUI(state), tags.SetupUI(state), emblems.SetupUI(state), decals.SetupUI(state), medals.SetupUI(state), pips.SetupUI(state), patterns.SetupUI(state),
	}

	contentStack := container.NewStack()
	navButtons := make([]*widget.Button, len(catNames))

	state.GenThumbBtn = widget.NewButtonWithIcon("Generate & Save Thumbnail", theme.FileImageIcon(), func() {
		data.GenerateAndSaveThumbnail(state, "", "", state.ThumbIdEntry.Text)
	})

	state.ReplaceBtn = widget.NewButtonWithIcon("Replace Texture", theme.FolderOpenIcon(), func() {
		if state.CurrentAssetSymbol == "" {
			dialog.ShowInformation("Error", "No asset symbol focused for replacement.", w)
			return
		}
		path, err := data.PickFile("PNG Files (*.png)|*.png|All Files (*.*)|*.*")
		if err == nil && path != "" {
			data.HandleTextureReplacement(state, state.CurrentAssetSymbol, path, state.ReplaceBtn, "Replacing texture...")
		}
	})

	state.MainPreviewGroup = container.NewVBox(
		container.NewHBox(layout.NewSpacer(), state.TextureImage, layout.NewSpacer()),
		container.NewHBox(layout.NewSpacer(), state.ReplaceBtn, layout.NewSpacer()),
	)

	selectTab := func(idx int) {
		state.ClearUI()
		contentStack.Objects = []fyne.CanvasObject{catUIs[idx]}
		contentStack.Refresh()

		// Update button styles (simulate active)
		for i, btn := range navButtons {
			if i == idx {
				btn.Importance = widget.HighImportance
			} else {
				btn.Importance = widget.LowImportance
			}
			btn.Refresh()
		}

		showThumbCard := false
		showTextureCard := false
		showThumbImage := true
		
		state.EmissivePreviewLabel.Hide()
		state.EmissivePreviewWrapper.Hide()
		state.GenThumbBtn.Hide()

		switch catNames[idx] {
		case "Tints":
			showThumbCard = true
			state.GenThumbBtn.Show()
		case "Emissives":
			showThumbCard = true
			state.EmissivePreviewLabel.Show()
			state.EmissivePreviewWrapper.Show()
		case "Fanfares":
			showThumbCard = true
			state.GenThumbBtn.Hide()
		case "Titles":
			showThumbImage = false
			state.ThumbIdItem.Widget.Hide()
		case "Emotes":
			showTextureCard = false
			showThumbImage = false
			state.ThumbIdItem.Widget.Hide()
		case "Banners", "Tags", "Emblems", "Decals", "Medals", "Pips", "Patterns":
			showTextureCard = false
			showThumbImage = false
			state.ThumbIdItem.Widget.Hide()
		}

		if showThumbCard { state.ThumbnailCard.Show() } else { state.ThumbnailCard.Hide() }
		if showTextureCard { state.MainPreviewCard.Show() } else { state.MainPreviewCard.Hide() }
		
		if showThumbImage { state.ThumbImage.Show() } else { state.ThumbImage.Hide() }

		if idx >= 0 && idx <= 4 {
			if state.PreviewTintContainer != nil { state.PreviewTintContainer.Hide() }
		} else {
			if state.PreviewTintContainer != nil { state.PreviewTintContainer.Show() }
		}
		
		if state.PreviewTintContainer != nil { state.PreviewTintContainer.Refresh() }
		if state.Window != nil && state.Window.Content() != nil {
			state.Window.Content().Refresh()
		}
	}

	for i, name := range catNames {
		idx := i
		navButtons[i] = widget.NewButtonWithIcon(name, catIcons[i], func() { selectTab(idx) })
	}

	row1 := container.NewGridWithColumns(5, navButtons[0], navButtons[1], navButtons[2], navButtons[3], navButtons[4])
	row2 := container.NewGridWithColumns(7, navButtons[5], navButtons[6], navButtons[7], navButtons[8], navButtons[9], navButtons[10], navButtons[11])
	navArea := container.NewVBox(row1, row2)

	// Action Bar Enhancement
	btnExtractAssets := widget.NewButtonWithIcon("EXTRACT ASSETS (TINTS/TEXTURES)", theme.DownloadIcon(), func() {
		loading := dialog.NewCustom("Extracting Assets...", "Cancel", widget.NewProgressBarInfinite(), w)
		loading.Show()
		go func() {
			err := data.RunExtract(state, state.Settings.EchoVRDataPath, "tints,textures")
			loading.Hide()
			if err != nil {
				dialog.ShowError(err, w)
			} else {
				dialog.ShowInformation("Success", "Tints and Textures extracted to pcvr-extracted folder.", w)
			}
		}()
	})

	btnRepack := widget.NewButtonWithIcon("REPACK PACKAGE", theme.StorageIcon(), func() {
		data.ShowRepackDialog(state)
	})
	btnRepack.Importance = widget.HighImportance

	btnSettings := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		extractedPath := widget.NewEntry(); extractedPath.SetText(state.Settings.ExtractedPath)
		echoPath := widget.NewEntry(); echoPath.SetText(state.Settings.EchoVRDataPath)
		cachePath := widget.NewEntry(); cachePath.SetText(state.Settings.TextureCachePath)

		makeBrowseItem := func(entry *widget.Entry) fyne.CanvasObject {
			browseBtn := widget.NewButton("Browse", func() {
				path, err := data.PickFolder()
				if err == nil && path != "" {
					entry.SetText(path)
				}
			})
			return container.NewBorder(nil, nil, nil, browseBtn, entry)
		}

		form := &widget.Form{
			Items: []*widget.FormItem{
				{Text: "Extracted Path", Widget: makeBrowseItem(extractedPath)},
				{Text: "EchoVR Path", Widget: makeBrowseItem(echoPath)},
				{Text: "Cache Path", Widget: makeBrowseItem(cachePath)},
			},
			OnSubmit: func() {
				state.Settings.ExtractedPath = extractedPath.Text
				state.Settings.EchoVRDataPath = echoPath.Text
				state.Settings.TextureCachePath = cachePath.Text
				saveSettings()
			},
		}

		modal := dialog.NewCustom("Editor Settings", "Close", container.NewPadded(form), w)
		modal.Resize(fyne.NewSize(600, 350))
		modal.Show()
	})

	btnLoadFile := widget.NewButtonWithIcon("Load", theme.FileIcon(), func() {
		path, err := data.PickFile("Data Files (*.dat)|*.dat|All Files (*.*)|*.*")
		if err == nil && path != "" {
			dataBytes, _ := os.ReadFile(path)
			cList, err := data.BytesToCosmeticList(dataBytes)
			if err != nil { dialog.ShowError(err, w); return }
			state.CosmeticList = cList
			state.RefreshIndices()
			state.ClearUI()
		}
	})

	btnChooseMode := widget.NewButton(state.Settings.Mode, func() {
		if state.Settings.Mode == "PCVR" { state.Settings.Mode = "Quest" } else { state.Settings.Mode = "PCVR" }
		saveSettings()
		w.SetTitle("Cosmetics Editor - " + state.Settings.Mode)
		w.Content().Refresh()
	})

	actionBar := container.NewBorder(nil, nil, nil, 
		container.NewHBox(btnChooseMode, btnLoadFile, btnSettings), 
		container.NewVBox(btnExtractAssets, btnRepack),
	)

	leftSide := container.NewBorder(
		container.NewVBox(container.NewPadded(actionBar), widget.NewSeparator()),
		nil, nil, nil,
		container.NewBorder(container.NewPadded(navArea), nil, nil, nil, contentStack),
	)

	// Sidebar Aesthetics
	state.EmissivePreviewImage = canvas.NewImageFromImage(nil)
	state.EmissivePreviewImage.FillMode = canvas.ImageFillContain
	state.EmissivePreviewImage.SetMinSize(fyne.NewSize(150, 150))
	state.EmissivePreviewWrapper = container.NewPadded(container.NewMax(
		canvas.NewRectangle(color.RGBA{22, 22, 22, 255}),
		state.EmissivePreviewImage,
	))
	state.EmissivePreviewLabel = widget.NewLabel("Color Preview")
	state.EmissivePreviewWrapper.Hide()
	state.EmissivePreviewLabel.Hide()

	_ = widget.NewFormItem("Thumbnail ID", state.ThumbIdEntry)
	state.ThumbIdItem = widget.NewFormItem("Thumbnail ID", state.ThumbIdEntry)
	
	state.ThumbnailCard = data.NewCard("Preview", container.NewVBox(
		container.NewCenter(state.ThumbImage),
		state.EmissivePreviewLabel,
		state.EmissivePreviewWrapper,
		state.GenThumbBtn,
	)).(*fyne.Container)

	state.MainPreviewCard = data.NewCard("Texture Replacement", container.NewVBox(
		container.NewHBox(layout.NewSpacer(), state.TextureImage, layout.NewSpacer()),
		container.NewHBox(layout.NewSpacer(), state.ReplaceBtn, layout.NewSpacer()),
	)).(*fyne.Container)

	sidebarContent := container.NewVBox(
		state.ThumbnailCard,
		data.NewCard("Info", container.NewVBox(
			state.NameEntry,
			state.DescEntry,
			state.ThumbIdEntry,
			widget.NewForm(widget.NewFormItem("Rarity", state.RaritySelect)),
		)),
		state.CategoryEditor,
		state.MainPreviewCard,
	)

	// Footer & Main Splitting
	state.PreviewTintCheck = widget.NewCheck("Preview with Tint", func(b bool) {
		if state.RefreshCurrent != nil { state.RefreshCurrent(state) }
	})
	state.PreviewTintSelect = widget.NewSelect([]string{}, func(s string) {
		for i, idx := range state.CategoryIndices["Tints"] {
			if strings.TrimRight(string(state.CosmeticList.CosmeticEntries[idx].CEntry.DisplayNameString[:]), "\x00") == s {
				state.PreviewTintIndex = i; break
			}
		}
		if state.RefreshCurrent != nil { state.RefreshCurrent(state) }
	})
	state.PreviewTintContainer = container.NewVBox(state.PreviewTintCheck, state.PreviewTintSelect)
	state.PreviewTintContainer.Hide()

	btnSave := widget.NewButtonWithIcon("SAVE DATA FILE", theme.DocumentSaveIcon(), func() {
		inputDir := data.InputDirNamePC
		tintFolder := data.TintFolderPC
		tintFile := data.TintFileNamePC
		if state.Settings.Mode == "Quest" {
			inputDir = data.InputDirNameQuest
			tintFolder = data.TintFolderQuest
			tintFile = data.TintFileNameQuest
		}

		dbDir := filepath.Join(data.GetSettingsDir(), inputDir, tintFolder)
		os.MkdirAll(dbDir, 0755)
		dbPath := filepath.Join(dbDir, tintFile)

		if err := state.HandleSave(dbPath); err == nil {
			dialog.ShowInformation("Success", "File saved correctly to:\n"+dbPath, w)
			// Still save to temp for safety
			state.HandleSave(tempFilePath)
		} else {
			dialog.ShowError(err, w)
		}
	})
	btnSave.Importance = widget.HighImportance

	footer := container.NewVBox(
		widget.NewSeparator(),
		container.NewPadded(container.NewVBox(state.PreviewTintContainer, btnSave, state.StatusLabel)),
	)

	rightSide := container.NewBorder(nil, footer, nil, nil, container.NewVScroll(sidebarContent))
	mainSplit := container.NewHSplit(leftSide, container.NewPadded(rightSide))
	mainSplit.Offset = 0.4
	w.SetContent(mainSplit)

	// Initial Load logic
	go func() {
		time.Sleep(200 * time.Millisecond)
		
		// Priority: 1. Input Database, 2. Autosave, 3. Embedded Original
		var b []byte
		inputDir := data.InputDirNamePC
		tintFolder := data.TintFolderPC
		tintFile := data.TintFileNamePC
		if state.Settings.Mode == "Quest" {
			inputDir = data.InputDirNameQuest
			tintFolder = data.TintFolderQuest
			tintFile = data.TintFileNameQuest
		}
		
		dbPath := filepath.Join(data.GetSettingsDir(), inputDir, tintFolder, tintFile)
		if data, err := os.ReadFile(dbPath); err == nil {
			b = data
			state.StatusLabel.SetText("Loaded from input database.")
		} else if data, err := os.ReadFile(tempFilePath); err == nil {
			b = data
			state.StatusLabel.SetText("Resumed from autosave.")
		} else {
			b = embeddedOriginal
			state.StatusLabel.SetText("Loaded default database.")
		}
		
		cList, _ := data.BytesToCosmeticList(b)
		state.CosmeticList = cList
		state.RefreshIndices()
		state.ClearUI()
		selectTab(0)
	}()

	w.ShowAndRun()
}

func loadSettings() {
	file, err := os.ReadFile(settingsFile)
	if err == nil { json.Unmarshal(file, &state.Settings) }
}

func saveSettings() {
	file, _ := json.MarshalIndent(state.Settings, "", "  ")
	os.WriteFile(settingsFile, file, 0644)
}
