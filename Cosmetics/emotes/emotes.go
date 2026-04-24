package emotes

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	data "evrCosmeticResearch/Data"
	"fmt"
	"image"
	"image/gif"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/nfnt/resize"
)

// CEmote holds the editable fields for a emote cosmetic entry.
type CEmote struct {
	InternalName    string
	DisplayName     string
	Description     string
	Rarity          int64
	ThumbnailSymbol int64
	Framerate       uint32

	Unk1        uint32
	EmoteFrames []string
}

// ToCosmeticEntry converts a CEmote to a raw CosmeticEntry for serialization.
func (c *CEmote) ToCosmeticEntry() (data.CosmeticEntry, error) {
	foo := data.CosmeticEntry{}
	foo.CEntry = data.NewCDescriptor()

	foo.CEntry.CosmeticTypeSymbol = int64(data.ToSymbol("emote"))
	foo.CEntry.InternalNameSymbol = int64(data.ToSymbol(strings.TrimSpace(c.InternalName)))
	foo.CEntry.InternalNameSymbol2 = foo.CEntry.InternalNameSymbol

	copy(foo.CEntry.InternalNameString[:], []byte(c.InternalName))
	copy(foo.CEntry.DisplayNameString[:], []byte(c.DisplayName))
	copy(foo.CEntry.DescriptionString[:], []byte(c.Description))

	foo.CEntry.RaritySymbol = c.Rarity
	foo.CEntry.ThumbnailSymbol = c.ThumbnailSymbol
	foo.CEntry.TextureSymbol = -1 // Emotes don't use TextureSymbol

	foo.CEntry.ImageListingEntrySize = int64(len(c.EmoteFrames) * 8)
	foo.CEntry.EmoteFrameCount = int64(len(c.EmoteFrames))
	foo.CEntry.EmoteFrameCount2 = foo.CEntry.EmoteFrameCount
	foo.CEntry.EmoteUnk2 = c.Framerate
	foo.CEntry.EmoteUnk1 = c.Unk1
	foo.CEntry.UnknownSymbol9 = 8287024585674271749

	foo.CEntryExtData = make([]byte, len(c.EmoteFrames)*8)
	for i, frame := range c.EmoteFrames {
		binary.LittleEndian.PutUint64(foo.CEntryExtData[i*8:], uint64(data.HexToSymbol(frame)))
	}

	return foo, nil
}

// FromCosmeticEntry populates a CEmote from a raw CosmeticEntry.
func (c *CEmote) FromCosmeticEntry(d data.CosmeticEntry) error {
	c.InternalName = string(bytes.TrimRight(d.CEntry.InternalNameString[:], "\x00"))
	c.DisplayName = string(bytes.TrimRight(d.CEntry.DisplayNameString[:], "\x00"))
	c.Description = string(bytes.TrimRight(d.CEntry.DescriptionString[:], "\x00"))
	c.Rarity = d.CEntry.RaritySymbol
	c.ThumbnailSymbol = d.CEntry.ThumbnailSymbol
	c.Framerate = d.CEntry.EmoteUnk2
	c.Unk1 = d.CEntry.EmoteUnk1

	c.EmoteFrames = make([]string, len(d.CEntryExtData)/8)
	for i := 0; i < len(d.CEntryExtData); i += 8 {
		c.EmoteFrames[i/8] = data.SymbolToHex(int64(binary.LittleEndian.Uint64(d.CEntryExtData[i : i+8])))
	}
	return nil
}

var (
	emoteList   *widget.List
	searchEntry *widget.Entry

	// GIF State
	selectedGifPath          string
	currentEmoteFrames       []string
	currentEmoteInternalName string
	gifFrameLabel            *widget.Label
	emoteReqFrameLabel       *widget.Label
	replaceGifBtn            *widget.Button
	gifPreviewImage          *canvas.Image
	gifAnimCancel            context.CancelFunc
)

// SetupUI builds and returns the Fyne canvas object for the Emotes tab.
func SetupUI(state *data.AppState) fyne.CanvasObject {
	searchEntry = widget.NewEntry()
	searchEntry.PlaceHolder = "Search Emotes..."
	searchEntry.OnChanged = func(s string) {
		RefreshFilter(state, s)
	}

	emoteList = widget.NewList(
		func() int {
			return len(state.CategoryFiltered["Emotes"])
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			realIdx := state.CategoryFiltered["Emotes"][id]
			entry := state.CosmeticList.CosmeticEntries[realIdx]
			dName := string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00"))
			item.(*widget.Label).SetText(dName)
		},
	)

	emoteList.OnSelected = func(id widget.ListItemID) {
		realIdx := state.CategoryFiltered["Emotes"][id]
		LoadToEditor(state, realIdx)
	}

	content := container.NewBorder(searchEntry, nil, nil, nil, emoteList)

	return content
}

func stopGifPreview() {
	if gifAnimCancel != nil {
		gifAnimCancel()
		gifAnimCancel = nil
	}
	if gifPreviewImage != nil {
		fyne.Do(func() {
			gifPreviewImage.Resource = nil
			gifPreviewImage.Image = nil
			gifPreviewImage.Refresh()
		})
	}
}

func startGifPreview(frames []image.Image) {
	stopGifPreview()
	if len(frames) == 0 {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	gifAnimCancel = cancel

	var resources []fyne.Resource
	for i, img := range frames {
		var buf bytes.Buffer
		png.Encode(&buf, img)
		resources = append(resources, fyne.NewStaticResource(fmt.Sprintf("frame%d.png", i), buf.Bytes()))
	}

	go func() {
		ticker := time.NewTicker(66 * time.Millisecond) // ~15fps
		defer ticker.Stop()
		idx := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				res := resources[idx%len(resources)]
				fyne.Do(func() {
					gifPreviewImage.Resource = res
					gifPreviewImage.Refresh()
				})
				idx++
			}
		}
	}()
}

// LoadToEditor populates the shared sidebar with the emote at the given CosmeticList index.
func LoadToEditor(state *data.AppState, realIdx int) {
	if realIdx < 0 || realIdx >= len(state.CosmeticList.CosmeticEntries) {
		return
	}
	state.SelectedIndex = realIdx
	state.SelectedCategory = "Emotes"

	t := CEmote{}
	if err := t.FromCosmeticEntry(state.CosmeticList.CosmeticEntries[realIdx]); err != nil {
		return
	}

	state.IsLoadingEntry = true
	state.NameEntry.SetText(t.DisplayName)
	state.DescEntry.SetText(t.Description)
	state.ThumbIdEntry.SetText(data.SymbolToHex(t.ThumbnailSymbol))
	state.RaritySelect.SetSelected(state.GetRarityName(t.Rarity))
	state.UpdateSidebarThumbnail(t.ThumbnailSymbol)
	state.UpdateMainTexture(0)

	fmt.Printf("[Emote] Loading '%s' with %d frames\n", t.DisplayName, len(t.EmoteFrames))

	currentEmoteFrames = t.EmoteFrames
	currentEmoteInternalName = t.InternalName
	selectedGifPath = ""

	// Ensure all frames are cached for preview
	for _, fSym := range t.EmoteFrames {
		data.EnsureTextureCached(state, fSym)
	}

	// Resolve cache path
	cacheDir := state.Settings.TextureCachePath
	if cacheDir == "" {
		cacheDir = filepath.Join(data.GetSettingsDir(), "texture_cache")
	}
	fmt.Printf("[Emote] Using cache directory: %s\n", cacheDir)

	// Animation setup
	if state.CancelAnim != nil {
		state.CancelAnim()
	}
	ctx, cancel := context.WithCancel(context.Background())
	state.CancelAnim = cancel

	go func(ctx context.Context) {
		ticker := time.NewTicker(time.Second / 15)
		defer ticker.Stop()
		frameIdx := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if len(t.EmoteFrames) == 0 {
					return
				}
				fSymbol := t.EmoteFrames[frameIdx%len(t.EmoteFrames)]
				path := filepath.Join(cacheDir, fSymbol+".png")
				if _, err := os.Stat(path); err != nil {
					// Try minimal hex
					symVal := data.HexToSymbol(fSymbol)
					fHex := data.SymbolToHex(symVal)
					minimalPath := filepath.Join(cacheDir, fHex+".png")
					if _, err := os.Stat(minimalPath); err == nil {
						path = minimalPath
					} else {
						if frameIdx%len(t.EmoteFrames) == 0 {
							fmt.Printf("[Emote] Preview failed to find frame 0. Tried:\n  - %s\n  - %s\n", path, minimalPath)
						}
					}
				}

				if _, err := os.Stat(path); err == nil {
					res, _ := fyne.LoadResourceFromPath(path)
					fyne.Do(func() {
						state.TextureImage.Resource = res
						state.TextureImage.Image = nil
						state.TextureImage.Refresh()
					})
				}
				frameIdx++
			}
		}
	}(ctx)

	rate := widget.NewEntry()
	rate.SetText(fmt.Sprintf("%d", t.Framerate))
	unk := widget.NewEntry()
	unk.SetText(fmt.Sprintf("%d", t.Unk1))

	saveEmote := func() {
		if state.IsLoadingEntry {
			return
		}
		entry := &state.CosmeticList.CosmeticEntries[state.SelectedIndex]
		v, _ := strconv.ParseUint(rate.Text, 10, 32)
		entry.CEntry.EmoteUnk2 = uint32(v)
		v, _ = strconv.ParseUint(unk.Text, 10, 32)
		entry.CEntry.EmoteUnk1 = uint32(v)
		entry.CEntry.EmoteFrameCount = int64(len(currentEmoteFrames))
		entry.CEntry.EmoteFrameCount2 = entry.CEntry.EmoteFrameCount
		entry.CEntry.ImageListingEntrySize = int64(len(currentEmoteFrames) * 8)
		entry.CEntryExtData = make([]byte, len(currentEmoteFrames)*8)
		for i, f := range currentEmoteFrames {
			binary.LittleEndian.PutUint64(entry.CEntryExtData[i*8:], uint64(data.HexToSymbol(f)))
		}
	}
	rate.OnChanged = func(string) { saveEmote() }
	unk.OnChanged = func(string) { saveEmote() }

	gifFrameLabel = widget.NewLabel("GIF Frames: 0")
	emoteReqFrameLabel = widget.NewLabel(fmt.Sprintf("Required: %d", len(t.EmoteFrames)))
	gifPreviewImage = canvas.NewImageFromResource(nil)
	gifPreviewImage.FillMode = canvas.ImageFillContain
	gifPreviewImage.SetMinSize(fyne.NewSize(0, 200))

	selectGifBtn := widget.NewButton("Select GIF", func() {
		path, err := data.PickFile("GIF Files (*.gif)|*.gif|All Files (*.*)|*.*")
		if err == nil && path != "" {
			selectedGifPath = path
			f, _ := os.Open(path)
			g, err := gif.DecodeAll(f)
			f.Close()
			if err != nil {
				dialog.ShowError(err, state.Window)
				return
			}

			gifFrameLabel.SetText(fmt.Sprintf("GIF Frames: %d", len(g.Image)))

			// Verbatim legacy auto-resize logic
			newCount := len(g.Image)
			oldCount := len(currentEmoteFrames)
			if newCount != oldCount {
				if newCount > oldCount {
					for i := oldCount; i < newCount; i++ {
						// Deterministic ID based on emote name and frame index
						newSymbol := data.ToSymbol(currentEmoteInternalName + "_frame_" + strconv.Itoa(i))
						currentEmoteFrames = append(currentEmoteFrames, data.SymbolToHex(int64(newSymbol)))
					}
				} else {
					currentEmoteFrames = currentEmoteFrames[:newCount]
				}
				emoteReqFrameLabel.SetText(fmt.Sprintf("Required: %d", len(currentEmoteFrames)))
			}

			replaceGifBtn.Enable()

			var frames []image.Image
			for _, img := range g.Image {
				frames = append(frames, img)
			}
			startGifPreview(frames)
		}
	})

	replaceGifBtn = widget.NewButton("Replace GIF", func() {
		if selectedGifPath == "" || len(currentEmoteFrames) == 0 {
			return
		}
		replaceGifBtn.Disable()
		state.StatusLabel.SetText("Processing GIF...")

		go func() {
			defer fyne.Do(func() { replaceGifBtn.Enable() })

			// legacy logic
			f, _ := os.Open(selectedGifPath)
			g, _ := gif.DecodeAll(f)
			f.Close()

			settingsPath := data.GetSettingsDir()
			tempDir := filepath.Join(settingsPath, "Temp")
			os.MkdirAll(tempDir, 0755)

			texconvPath, err := data.FindTool(settingsPath, "texconv.exe")
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, state.Window) })
				return
			}

			extPath := state.Settings.ExtractedPath
			if extPath == "" {
				extPath = filepath.Join(settingsPath, data.ExtractedDirName)
			}

			processedCount := 0
			var missingAssets []string

			// Verbatim legacy loop
			for i, img := range g.Image {
				if i >= len(currentEmoteFrames) {
					break
				}
				idStr := currentEmoteFrames[i]

				// legacy path resolution
				srcFile := filepath.Join(extPath, "4a4c32c49300b8a0", idStr)
				if _, err := os.Stat(srcFile); os.IsNotExist(err) {
					altPath := filepath.Join(extPath, idStr)
					if _, err := os.Stat(altPath); err == nil {
						srcFile = altPath
					} else {
						// template fallback from index 0
						id0Str := currentEmoteFrames[0]
						srcFile = filepath.Join(extPath, "4a4c32c49300b8a0", id0Str)
						if _, err := os.Stat(srcFile); os.IsNotExist(err) {
							srcFile = filepath.Join(extPath, id0Str)
						}
					}
				}

				if _, err := os.Stat(srcFile); os.IsNotExist(err) {
					missingAssets = append(missingAssets, idStr)
					continue
				}

				// Verbatim legacy header-patching logic
				header := make([]byte, 400)
				fSrc, _ := os.Open(srcFile)
				n, _ := fSrc.Read(header)
				fSrc.Close()
				if n < 400 {
					missingAssets = append(missingAssets, idStr+" (small)")
					continue
				}

				origHeight := binary.LittleEndian.Uint32(header[268:])
				origWidth := binary.LittleEndian.Uint32(header[272:])
				resizedImg := resize.Resize(uint(origWidth), uint(origHeight), img, resize.Lanczos3)

				pngPath := filepath.Join(tempDir, fmt.Sprintf("frame_%d_resized.png", i))
				pngOut, _ := os.Create(pngPath)
				png.Encode(pngOut, resizedImg)
				pngOut.Close()

				ddsPath := filepath.Join(tempDir, fmt.Sprintf("frame_%d.dds", i))
				cmd := exec.Command(texconvPath, "encode", pngPath, ddsPath)
				cmd.SysProcAttr = data.HiddenProcAttr()
				cmd.CombinedOutput()

				ddsData, _ := os.ReadFile(ddsPath)
				ddsSize := uint32(len(ddsData))

				origMeta, _ := os.ReadFile(srcFile)
				headerOnly := make([]byte, 256)
				copy(headerOnly, origMeta[:256])
				binary.LittleEndian.PutUint64(headerOnly[64:72], uint64(data.HexToSymbol(idStr)))
				binary.LittleEndian.PutUint32(headerOnly[244:248], ddsSize)
				combinedAsset := append(headerOnly, ddsData...)

				// direct filesystem writes
				inputDirName := data.InputDirNamePC
				if state.Settings.Mode == "Quest" {
					inputDirName = data.InputDirNameQuest
				}
				absInputDir := filepath.Join(settingsPath, inputDirName)
				metaOutDir := filepath.Join(absInputDir, "4a4c32c49300b8a0")
				texOutDir := filepath.Join(absInputDir, "beac1969cb7b8861")
				os.MkdirAll(metaOutDir, 0755)
				os.MkdirAll(texOutDir, 0755)

				os.WriteFile(filepath.Join(metaOutDir, idStr), combinedAsset, 0644)
				os.WriteFile(filepath.Join(texOutDir, idStr), make([]byte, 16), 0644)

				processedCount++

				os.Remove(pngPath)
				os.Remove(ddsPath)
			}

			fyne.Do(func() {
				if processedCount > 0 {
					// Verbatim commit to list
					realIdx := state.SelectedIndex
					t := CEmote{}
					if err := t.FromCosmeticEntry(state.CosmeticList.CosmeticEntries[realIdx]); err == nil {
						t.EmoteFrames = currentEmoteFrames
						if newEntry, err := t.ToCosmeticEntry(); err == nil {
							state.CosmeticList.CosmeticEntries[realIdx] = newEntry
						}
					}

					// Auto-save to database path to ensure persistence
					inputDir := data.InputDirNamePC
					tintFolder := data.TintFolderPC
					tintFile := data.TintFileNamePC
					if state.Settings.Mode == "Quest" {
						inputDir = data.InputDirNameQuest
						tintFolder = data.TintFolderQuest
						tintFile = data.TintFileNameQuest
					}
					dbPath := filepath.Join(data.GetSettingsDir(), inputDir, tintFolder, tintFile)
					state.HandleSave(dbPath)

					state.StatusLabel.SetText(fmt.Sprintf("GIF Replaced: %d/%d processed", processedCount, len(g.Image)))
					dialog.ShowInformation("Success", "GIF repacked successfully.", state.Window)
				} else {
					dialog.ShowError(errors.New("Failed to process frames."), state.Window)
				}
			})
		}()
	})
	replaceGifBtn.Disable()

	state.CategoryEditor.Objects = []fyne.CanvasObject{
		widget.NewForm(
			widget.NewFormItem("Framerate", rate),
			widget.NewFormItem("Unk1", unk),
			widget.NewFormItem("Frame Count", widget.NewLabel(fmt.Sprintf("%d", len(t.EmoteFrames)))),
		),
		widget.NewLabel("Original Emote Preview"),
		container.NewCenter(state.TextureImage),
		widget.NewSeparator(),
		widget.NewLabel("Custom GIF Replacement"),
		selectGifBtn,
		container.NewHBox(emoteReqFrameLabel, layout.NewSpacer(), gifFrameLabel),
		gifPreviewImage,
		replaceGifBtn,
	}
	state.CategoryEditor.Refresh()

	state.IsLoadingEntry = false
}

// RefreshFilter re-filters the emotes list to entries whose display name contains query.
func RefreshFilter(state *data.AppState, query string) {
	query = strings.ToLower(query)
	state.CategoryFiltered["Emotes"] = []int{}
	for _, idx := range state.CategoryIndices["Emotes"] {
		entry := state.CosmeticList.CosmeticEntries[idx]
		dName := strings.ToLower(string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00")))
		if query == "" || strings.Contains(dName, query) {
			state.CategoryFiltered["Emotes"] = append(state.CategoryFiltered["Emotes"], idx)
		}
	}
	if emoteList != nil {
		emoteList.Refresh()
	}
}
