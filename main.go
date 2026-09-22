package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/EchoTools/cosmetic-editor/Cosmetics/banners"
	"github.com/EchoTools/cosmetic-editor/Cosmetics/boosters"
	"github.com/EchoTools/cosmetic-editor/Cosmetics/bracers"
	"github.com/EchoTools/cosmetic-editor/Cosmetics/chassis"
	"github.com/EchoTools/cosmetic-editor/Cosmetics/decals"
	"github.com/EchoTools/cosmetic-editor/Cosmetics/emblems"
	"github.com/EchoTools/cosmetic-editor/Cosmetics/emissives"
	"github.com/EchoTools/cosmetic-editor/Cosmetics/emotes"
	"github.com/EchoTools/cosmetic-editor/Cosmetics/fanfares"
	"github.com/EchoTools/cosmetic-editor/Cosmetics/medals"
	"github.com/EchoTools/cosmetic-editor/Cosmetics/patterns"
	"github.com/EchoTools/cosmetic-editor/Cosmetics/pips"
	"github.com/EchoTools/cosmetic-editor/Cosmetics/tags"
	"github.com/EchoTools/cosmetic-editor/Cosmetics/tints"
	"github.com/EchoTools/cosmetic-editor/Cosmetics/titles"
	data "github.com/EchoTools/cosmetic-editor/Data"
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
	// startupDone is closed once the initial database load has finished.
	startupDone chan struct{}

	settingsFile string
	tempFilePath string
)

func fixEchoVRPath(p string) string {
	idx := strings.Index(strings.ToLower(p), "ready-at-dawn-echo-arena")
	if idx != -1 {
		base := p[:idx+len("ready-at-dawn-echo-arena")]
		return filepath.Join(base, "_data", "5932408047", "rad15", "win10")
	}
	return p
}

func main() {
	os.Setenv("FYNE_GL_VERSION", "2.1")

	// The app has to exist before the storage layout is decided: on Android
	// the only writable location is the one the app itself is given, and it is
	// reachable only through the app object.
	a := app.NewWithID("com.echotools.cosmeticeditor")
	a.SetIcon(fyne.NewStaticResource("icon.ico", embeddedIcon))

	// 1. SETUP: Directories & Settings
	if data.IsAndroid() {
		if root := a.Storage().RootURI(); root != nil {
			data.SetSettingsDir(filepath.Join(root.Path(), "settings"))
		}
	}
	settingsPath := data.GetSettingsDir()
	os.MkdirAll(settingsPath, 0755)

	tempDir := filepath.Join(settingsPath, "Temp")
	os.MkdirAll(tempDir, 0755)
	tempFilePath = filepath.Join(tempDir, "temp_autosave.dat")
	installModelScripts(tempDir)

	if data.IsAndroid() {
		// There is nowhere beside the binary to keep settings on a headset.
		settingsFile = filepath.Join(settingsPath, "settings.json")
	} else {
		exePath, _ := os.Executable()
		exeDir := filepath.Dir(exePath)
		if strings.Contains(strings.ToLower(exeDir), "go-build") || strings.Contains(strings.ToLower(exeDir), "temp") {
			cwd, _ := os.Getwd()
			exeDir = cwd
		}
		settingsFile = filepath.Join(exeDir, "settings.json")
	}

	w := a.NewWindow("EchoVR Cosmetics Editor")

	// Everything that touches widgets has to run on Fyne's UI thread. On
	// Android main() is not that thread, and building the UI from here made
	// Fyne report a call-thread error for nearly every widget and race the
	// renderer, so the UI is built once the app has started instead.
	a.Lifecycle().SetOnStarted(func() {
		buildUI(a, w, settingsPath)
		w.Show()
	})

	// Coming back to the app, typically from the All files access screen,
	// re-checks access and looks for the game's data again.
	if data.IsAndroid() {
		a.Lifecycle().SetOnEnteredForeground(func() {
			if state != nil && state.CosmeticList.CosmeticEntries != nil {
				checkQuestSetup()
			}
		})
	}

	// Save whenever the app leaves the foreground. Most editors only change the
	// database in memory and rely on Repack to write it, and a headset can kill
	// a backgrounded app (it does when it sleeps), which would lose those edits.
	a.Lifecycle().SetOnExitedForeground(persistEdits)

	a.Run()
}

// persistEdits writes the in-memory cosmetic database to the staging folder
// and the autosave file, so the next launch resumes from it.
func persistEdits() {
	// Never write before the database has loaded: that would save an empty
	// list over the real one.
	if state == nil || len(state.CosmeticList.CosmeticEntries) == 0 {
		return
	}
	if err := state.SaveCosmeticDB(); err != nil {
		fmt.Fprintf(os.Stderr, "saving edits: %v\n", err)
	}
	if err := state.HandleSave(tempFilePath); err != nil {
		fmt.Fprintf(os.Stderr, "saving autosave: %v\n", err)
	}
}

func loadSettings() {
	file, err := os.ReadFile(settingsFile)
	if err == nil {
		if err := json.Unmarshal(file, &state.Settings); err != nil && state.StatusLabel != nil {
			// Settings load before the status bar exists; a malformed file
			// must not crash startup.
			state.StatusLabel.SetText("Warning: failed to parse settings: " + err.Error())
		}
	}
}

func saveSettings() {
	file, _ := json.MarshalIndent(state.Settings, "", "  ")
	os.WriteFile(settingsFile, file, 0644)
}

// buildUI creates the window content and starts loading the cosmetic
// database. It runs on the UI thread, from the app's OnStarted callback.
func buildUI(a fyne.App, w fyne.Window, settingsPath string) {
	if !fyne.CurrentDevice().IsMobile() {
		// A headset or phone window is sized by the system, not the app.
		w.Resize(fyne.NewSize(1200, 850))
	}

	state = data.NewAppState(a, w)
	loadSettings()

	// 2. STABILIZE PATHS
	if data.IsAndroid() {
		// On the headset this is a Quest install by definition, and the game's
		// data sits at a known place under Android/media.  Detect it every
		// launch rather than trusting a saved path, which may predate a
		// reinstall.
		state.Settings.Mode = "Quest"
		if p := data.FindQuestDataPath(); p != "" {
			state.Settings.EchoVRDataPath = p
		}
	}
	if state.Settings.EchoVRDataPath == "" {
		state.Settings.EchoVRDataPath = data.DefaultDataPath(state.Platform())
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
		if state.SelectedIndex == -1 || state.IsLoadingEntry {
			return
		}
		entry := &state.CosmeticList.CosmeticEntries[state.SelectedIndex]
		entry.CEntry.RaritySymbol = state.GetRaritySymbol(s)
		state.AutoSave()
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
		if state.SelectedIndex == -1 || state.IsLoadingEntry {
			return
		}
		entry := &state.CosmeticList.CosmeticEntries[state.SelectedIndex]
		for i := range entry.CEntry.DisplayNameString {
			entry.CEntry.DisplayNameString[i] = 0
		}
		copy(entry.CEntry.DisplayNameString[:], []byte(s))
		state.AutoSave()
	}
	state.DescEntry.OnChanged = func(s string) {
		if state.SelectedIndex == -1 || state.IsLoadingEntry {
			return
		}
		entry := &state.CosmeticList.CosmeticEntries[state.SelectedIndex]
		for i := range entry.CEntry.DescriptionString {
			entry.CEntry.DescriptionString[i] = 0
		}
		copy(entry.CEntry.DescriptionString[:], []byte(s))
		state.AutoSave()
	}
	state.ThumbIdEntry.OnChanged = func(s string) {
		if state.SelectedIndex == -1 || state.IsLoadingEntry {
			return
		}
		entry := &state.CosmeticList.CosmeticEntries[state.SelectedIndex]
		entry.CEntry.ThumbnailSymbol = data.HexToSymbol(s)
		state.AutoSave()
	}

	state.CategoryEditor = container.NewVBox()

	// 2. NAVIGATION & CONTENT ASSEMBLY
	type category struct {
		name  string
		icon  fyne.Resource
		setup func(*data.AppState) fyne.CanvasObject
		// pcOnly categories edit 3D model or audio data. The Quest build
		// cannot change either, so those tabs are not offered there.
		pcOnly bool
	}
	allCategories := []category{
		{"Chassis", theme.AccountIcon(), chassis.SetupUI, true},
		{"Bracers", theme.AccountIcon(), bracers.SetupUI, true},
		{"Boosters", theme.AccountIcon(), boosters.SetupUI, true},
		{"Tints", theme.ColorPaletteIcon(), tints.SetupUI, false},
		{"Titles", theme.DocumentIcon(), titles.SetupUI, false},
		{"Emissives", theme.VisibilityIcon(), emissives.SetupUI, false},
		{"Fanfares", theme.VolumeUpIcon(), fanfares.SetupUI, true},
		{"Emotes", theme.MediaPlayIcon(), emotes.SetupUI, false},
		{"Banners", theme.GridIcon(), banners.SetupUI, false},
		{"Tags", theme.ContentCopyIcon(), tags.SetupUI, false},
		{"Emblems", theme.ComputerIcon(), emblems.SetupUI, false},
		{"Decals", theme.CheckButtonIcon(), decals.SetupUI, false},
		{"Medals", theme.HelpIcon(), medals.SetupUI, false},
		{"Pips", theme.MenuIcon(), pips.SetupUI, false},
		{"Patterns", theme.ListIcon(), patterns.SetupUI, false},
	}

	var catNames []string
	var catIcons []fyne.Resource
	var catUIs []fyne.CanvasObject
	for _, c := range allCategories {
		if c.pcOnly && state.Platform() == data.PlatformQuest {
			continue
		}
		catNames = append(catNames, c.name)
		catIcons = append(catIcons, c.icon)
		catUIs = append(catUIs, c.setup(state))
	}

	contentStack := container.NewStack()
	navButtons := make([]*widget.Button, len(catNames))

	state.GenThumbBtn = widget.NewButtonWithIcon("Generate & Save Thumbnail", theme.FileImageIcon(), func() {
		// Draw the thumbnail in the selected tint's own colours. This used to
		// pass none, so every generated thumbnail came out plain grey.
		if state.SelectedCategory != "Tints" || state.SelectedIndex < 0 {
			dialog.ShowInformation("Generate Thumbnail", "Select a tint first.", w)
			return
		}
		first, second, ok := data.TintThumbnailColors(state.CosmeticList.CosmeticEntries[state.SelectedIndex])
		if !ok {
			dialog.ShowError(fmt.Errorf("this tint has no colour data"), w)
			return
		}
		data.GenerateAndSaveThumbnail(state, data.ColorHex(first), data.ColorHex(second), state.ThumbIdEntry.Text)
	})

	state.ReplaceBtn = widget.NewButtonWithIcon("Replace Texture", theme.FolderOpenIcon(), func() {
		if state.CurrentAssetSymbol == "" {
			dialog.ShowInformation("Error", "No asset symbol focused for replacement.", w)
			return
		}
		data.PickFile(state, []string{".png"}, func(path string) {
			data.HandleTextureReplacement(state, state.CurrentAssetSymbol, path, state.ReplaceBtn, "Replacing texture...")
		})
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
		case "Chassis", "Bracers", "Boosters":
			// Generate Thumbnail recolours the tint template, so it only
			// makes sense for tints.
			showThumbCard = true
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

		if showThumbCard {
			state.ThumbnailCard.Show()
		} else {
			state.ThumbnailCard.Hide()
		}
		if showTextureCard {
			state.MainPreviewCard.Show()
		} else {
			state.MainPreviewCard.Hide()
		}

		if showThumbImage {
			state.ThumbImage.Show()
		} else {
			state.ThumbImage.Hide()
		}

		switch catNames[idx] {
		case "Banners", "Tags", "Emblems", "Decals", "Pips", "Patterns":
			if state.PreviewTintContainer != nil {
				state.PreviewTintContainer.Show()
			}
		default:
			if state.PreviewTintContainer != nil {
				state.PreviewTintContainer.Hide()
			}
		}

		if state.PreviewTintContainer != nil {
			state.PreviewTintContainer.Refresh()
		}
		if state.Window != nil && state.Window.Content() != nil {
			state.Window.Content().Refresh()
		}
	}

	for i, name := range catNames {
		idx := i
		navButtons[i] = widget.NewButtonWithIcon(name, catIcons[i], func() { selectTab(idx) })
	}

	// Lay the tabs out in rows narrow enough for their labels to fit. A
	// headset panel is much narrower than a desktop window, where eight across
	// clips every label.
	navCols := 8
	if fyne.CurrentDevice().IsMobile() {
		navCols = 4
	}
	navObjs := make([]fyne.CanvasObject, len(navButtons))
	for i, b := range navButtons {
		navObjs[i] = b
	}
	navArea := container.NewGridWithColumns(navCols, navObjs...)

	btnRepack := widget.NewButtonWithIcon("REPACK PACKAGE", theme.StorageIcon(), func() {
		data.ShowRepackDialog(state)
	})
	btnRepack.Importance = widget.HighImportance

	btnSettings := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		echoPath := widget.NewEntry()
		echoPath.SetText(state.Settings.EchoVRDataPath)
		cachePath := widget.NewEntry()
		cachePath.SetText(state.Settings.TextureCachePath)

		makeBrowseItem := func(entry *widget.Entry) fyne.CanvasObject {
			browseBtn := widget.NewButton("Browse", func() {
				data.PickFolder(state, "Select Folder", func(path string) {
					entry.SetText(fixEchoVRPath(path))
				})
			})
			return container.NewBorder(nil, nil, nil, browseBtn, entry)
		}

		form := &widget.Form{
			Items: []*widget.FormItem{
				{Text: "EchoVR Path", Widget: makeBrowseItem(echoPath)},
				{Text: "Cache Path", Widget: makeBrowseItem(cachePath)},
			},
			OnSubmit: func() {
				state.Settings.EchoVRDataPath = echoPath.Text
				if p := data.ResolveDataPath(echoPath.Text, state.Platform()); p != "" {
					state.Settings.EchoVRDataPath = p
				}
				state.Settings.TextureCachePath = cachePath.Text
				saveSettings()
			},
		}

		btnInstallMod := widget.NewButtonWithIcon("Install other mods", theme.ContentAddIcon(), installOtherMod)
		modsHelp := widget.NewLabel("Add a mod made with other tools (a .zip). Its files are applied the next time you repack.")
		modsHelp.Wrapping = fyne.TextWrapWord

		modal := dialog.NewCustom("Editor Settings", "Close", container.NewPadded(container.NewVBox(
			form,
			widget.NewSeparator(),
			modsHelp,
			btnInstallMod,
		)), w)
		modal.Resize(fyne.NewSize(600, 420))
		modal.Show()
	})

	btnResetData := widget.NewButtonWithIcon("Reset", theme.HistoryIcon(), func() {
		dialog.ShowConfirm("Reset to Stock", "Are you sure you want to reset all cosmetics back to the original stock data?\nThis cannot be undone.", func(b bool) {
			if b {
				cList, err := data.BytesToCosmeticList(embeddedOriginal)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to parse original stock data: %v", err), w)
					return
				}

				state.CosmeticList = cList
				state.AutoSave()

				state.RefreshIndices()
				state.ClearUI()
				selectTab(0)
				dialog.ShowInformation("Reset Complete", "The cosmetic data has been restored to the original stock settings.", w)
			}
		}, w)
	})

	btnChooseMode := widget.NewButton(state.Settings.Mode, func() {})
	btnChooseMode.Hide() // Quest isn't finished yet

	actionBar := container.NewBorder(nil, nil, nil,
		container.NewHBox(btnChooseMode, btnResetData, btnSettings),
		container.NewVBox(btnRepack),
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
		if state.RefreshCurrent != nil {
			state.RefreshCurrent(state)
		}
	})
	state.PreviewTintSelect = widget.NewSelect(nil, func(s string) {
		if s == "" {
			state.PreviewTintIndex = -1
		} else {
			for _, idx := range state.CategoryIndices["Tints"] {
				entry := state.CosmeticList.CosmeticEntries[idx]
				dName := strings.TrimRight(string(entry.CEntry.DisplayNameString[:]), "\x00")
				if dName == s {
					state.PreviewTintIndex = idx
					break
				}
			}
		}
		if state.RefreshCurrent != nil {
			state.RefreshCurrent(state)
		}
	})
	state.PreviewTintSelect.PlaceHolder = "Select a tint to preview"

	state.PreviewTintContainer = container.NewBorder(nil, nil, state.PreviewTintCheck, nil, state.PreviewTintSelect)
	state.PreviewTintContainer.Hide()

	footer := container.NewVBox(
		widget.NewSeparator(),
		container.NewPadded(container.NewVBox(state.PreviewTintContainer, state.StatusLabel)),
	)

	// Close Intercept
	w.SetCloseIntercept(func() {
		if state.NeedsRepack {
			dialog.ShowConfirm("Unsaved Changes", "You didn't repack your package, are you sure you want to exit?", func(b bool) {
				if b {
					w.Close()
				} else {
					// Flash repack button
					btnRepack.Importance = widget.WarningImportance
					btnRepack.Refresh()
					go func() {
						time.Sleep(1 * time.Second)
						btnRepack.Importance = widget.HighImportance
						btnRepack.Refresh()
					}()
				}
			}, w)
		} else {
			w.Close()
		}
	})

	rightSide := container.NewBorder(nil, footer, nil, nil, container.NewVScroll(sidebarContent))
	mainSplit := container.NewHSplit(leftSide, container.NewPadded(rightSide))
	mainSplit.Offset = 0.4
	w.SetContent(mainSplit)

	// Initial Load logic
	done := make(chan struct{})
	startupDone = done
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
			fyne.Do(func() { state.StatusLabel.SetText("Loaded from input database.") })
		} else if data, err := os.ReadFile(tempFilePath); err == nil {
			b = data
			fyne.Do(func() { state.StatusLabel.SetText("Resumed from autosave.") })
		} else {
			b = embeddedOriginal
			fyne.Do(func() { state.StatusLabel.SetText("Loaded default database.") })
		}

		cList, err := data.BytesToCosmeticList(b)
		if err != nil {
			fyne.Do(func() { state.StatusLabel.SetText("Warning: failed to parse cosmetic data: " + err.Error()) })
		}
		state.CosmeticList = cList
		fyne.Do(func() {
			state.RefreshIndices()
			state.ClearUI()
			selectTab(0)

			// Nothing is extracted: the database is built in and textures are
			// read from the package one at a time.  All the editor needs is to
			// know where the game's data directory is, so check that.
			if data.IsAndroid() {
				checkQuestSetup()
			} else if !data.HasManifest(state.Settings.EchoVRDataPath) {
				dialog.ShowConfirm("Setup Required", "Select your Echo VR folder so changes can be repacked into it.", func(b bool) {
					if !b {
						return
					}
					data.PickFolder(state, "Select Echo VR Folder", func(path string) {
						resolved := data.ResolveDataPath(path, state.Platform())
						if resolved == "" {
							dialog.ShowError(fmt.Errorf("no Echo VR %s data found under %s", state.Platform(), path), w)
							return
						}
						state.Settings.EchoVRDataPath = resolved
						saveSettings()
						data.WarmPackageReader(resolved)
					})
				}, w)
			} else {
				// Parse the package index now rather than on the first preview.
				data.WarmPackageReader(state.Settings.EchoVRDataPath)
			}
			close(done)
		})
	}()

}
