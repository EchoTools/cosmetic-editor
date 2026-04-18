package emotes

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"evrCosmeticResearch/Data"
	"fmt"
	"image"
	"image/gif"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/nfnt/resize"
)

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

func LoadToEditor(state *data.AppState, realIdx int) {
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
	state.RaritySelect.SetSelected(data.RaritySymbolToName[t.Rarity])
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
						if frameIdx % len(t.EmoteFrames) == 0 {
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

			newCount := len(g.Image)
			oldCount := len(currentEmoteFrames)
			if newCount != oldCount {
				if newCount > oldCount {
					for i := oldCount; i < newCount; i++ {
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

			f, _ := os.Open(selectedGifPath)
			g, _ := gif.DecodeAll(f)
			f.Close()

			settingsPath := data.GetSettingsDir()
			tempDir := filepath.Join(settingsPath, "Temp")
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
			templatePath := ""

			// 1. Resolve template from original frames if possible
			// We need the original symbols to find the template
			tOrig := CEmote{}
			tOrig.FromCosmeticEntry(state.CosmeticList.CosmeticEntries[state.SelectedIndex])
			for _, fSym := range tOrig.EmoteFrames {
				sym := data.HexToSymbol(fSym)
				templatePath = data.FindExtractedAsset(extPath, sym, data.ThumbMetaFolderPC)
				if templatePath == "" {
					templatePath = data.FindExtractedAsset(extPath, sym, data.TexMetaFolderPC)
				}
				if templatePath != "" {
					fmt.Printf("[Emote] Found template using original frame %s: %s\n", fSym, templatePath)
					break
				}
			}

			if templatePath == "" {
				fmt.Printf("[Emote] WARNING: No template found in original frames. Search will continue per frame.\n")
			}

			for i, img := range g.Image {
				idStr := currentEmoteFrames[i]
				symVal := data.HexToSymbol(idStr)
				
				fmt.Printf("Processing frame %d/%d (ID: %s)...\n", i+1, len(g.Image), idStr)

				// Use robust lookup to find source metadata
				srcFile := data.FindExtractedAsset(extPath, symVal, data.ThumbMetaFolderPC)
				if srcFile == "" {
					srcFile = data.FindExtractedAsset(extPath, symVal, data.TexMetaFolderPC)
				}

				if srcFile == "" {
					// Fallback to template (first frame)
					if templatePath != "" {
						srcFile = templatePath
					} else {
						fmt.Printf("Skipping frame %d: No metadata source found for %s\n", i, idStr)
						continue
					}
				}

				// Header and Resizing
				header := make([]byte, 400)
				fSrc, err := os.Open(srcFile)
				if err != nil {
					fmt.Printf("[Emote] ERROR: Could not open source %s: %v\n", srcFile, err)
					continue
				}
				n, _ := fSrc.Read(header)
				fSrc.Close()
				if n < 400 {
					fmt.Printf("[Emote] ERROR: Source %s too small (%d bytes)\n", srcFile, n)
					continue
				}

				origH := binary.LittleEndian.Uint32(header[268:272])
				origW := binary.LittleEndian.Uint32(header[272:276])
				fmt.Printf("[Emote] Frame %d: Original Dimensions %dx%d\n", i, origW, origH)

				resized := resize.Resize(uint(origW), uint(origH), img, resize.Lanczos3)

				pngPath := filepath.Join(tempDir, fmt.Sprintf("frame_%d.png", i))
				fPng, _ := os.Create(pngPath)
				png.Encode(fPng, resized)
				fPng.Close()

				ddsPath := filepath.Join(tempDir, fmt.Sprintf("frame_%d.dds", i))
				cmd := exec.Command(texconvPath, "encode", pngPath, ddsPath)
				cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
				if out, err := cmd.CombinedOutput(); err != nil {
					fmt.Printf("[Emote] ERROR: texconv failed on frame %d: %v\n%s\n", i, err, string(out))
					continue
				}

				ddsData, _ := os.ReadFile(ddsPath)
				ddsSize := uint32(len(ddsData))
				fmt.Printf("[Emote] Frame %d: Encoded DDS Size %d bytes\n", i, ddsSize)

				// Combine Header + DDS
				metaData, _ := os.ReadFile(srcFile)
				headerOnly := make([]byte, 256)
				copy(headerOnly, metaData[:256])
				binary.LittleEndian.PutUint64(headerOnly[64:72], uint64(data.HexToSymbol(idStr)))
				binary.LittleEndian.PutUint32(headerOnly[244:248], ddsSize)
				combined := append(headerOnly, ddsData...)

				metaOutDir := filepath.Join(settingsPath, data.InputDirNamePC, data.ThumbMetaFolderPC)
				texOutDir := filepath.Join(settingsPath, data.InputDirNamePC, data.ThumbTexFolderPC)
				os.MkdirAll(metaOutDir, 0755)
				os.MkdirAll(texOutDir, 0755)

				metaPath := filepath.Join(metaOutDir, idStr)
				texPath := filepath.Join(texOutDir, idStr)

				absMeta, _ := filepath.Abs(metaPath)

				if err := os.WriteFile(metaPath, combined, 0644); err != nil {
					fmt.Printf("[Emote] ERROR: Could not write meta for frame %d: %v\n", i, err)
					continue
				}
				if err := os.WriteFile(texPath, make([]byte, 16), 0644); err != nil {
					fmt.Printf("[Emote] ERROR: Could not write tex stub for frame %d: %v\n", i, err)
					continue
				}

				processedCount++
				fmt.Printf("[Emote] Saved frame %d to: %s\n", i, absMeta)
				os.Remove(pngPath)
				os.Remove(ddsPath)
			}

			fyne.Do(func() {
				if processedCount > 0 {
					saveEmote()
					state.HandleSave(filepath.Join(tempDir, "temp_autosave.dat"))
					dialog.ShowInformation("Success", fmt.Sprintf("Repacked %d frames.", processedCount), state.Window)
					state.StatusLabel.SetText(fmt.Sprintf("GIF Replaced: %d frames", processedCount))
				} else {
					dialog.ShowError(errors.New("No frames processed. Check extracted assets."), state.Window)
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
