package main

//go:generate rsrc -ico icon.ico

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// --- EMBEDDED FILES ---
//go:embed 0x43934c379cf1e366_original
var embeddedOriginal []byte

//go:embed icon.ico
var embeddedIcon []byte

//go:embed template_thumb.png
var embeddedTemplate []byte

// --- SETTINGS STRUCT ---
type AppSettings struct {
	AssetsPath      string `json:"assets_path"`
	TextureOutPath  string `json:"texture_out_path"`
	MetadataOutPath string `json:"metadata_out_path"`
	EchoVRDataPath  string `json:"echovr_data_path"`
	BackupPath      string `json:"backup_path"`
}

var (
	currentCList      cosmeticList
	tintIndices       []int
	filteredIndices   []int
	selectedListIndex int = -1

	isLoadingEntry bool


	tempFilePath string 
	settingsFile = "settings.json"
	appSettings  AppSettings
)

const (
	DefaultEchoPath = "./ready-at-dawn-echo-arena/_data/5932408047/rad15/win10"
	PackageName     = "48037dc70b0ecab2"

	// Base Directories
	SettingsDir     = "Settings"
	ExtractedDir    = "Settings/pcvr-extracted"
	InputDir        = "Settings/input-pcvr"
	OutputDir       = "Settings/output-both"
	BackupDirName   = "Backup" // Folder name inside Settings

	// 1. TINT FILE (Cosmetic List)
	// Path: Settings/input-pcvr/0/3671295590506143214/4869319423857648486
	TintFolder      = "0/3671295590506143214"
	TintFileName    = "4869319423857648486"

	// 2. THUMBNAIL TEXTURE (DDS)
	// Path: Settings/input-pcvr/0/-4707359568332879775/[ID]
	ThumbTexFolder  = "0/-4707359568332879775"

	// 3. THUMBNAIL METADATA (Hex)
	// Path: Settings/input-pcvr/0/5353709876897953952/[ID]
	ThumbMetaFolder = "0/5353709876897953952"
)

func main() {
	os.Setenv("FYNE_GL_VERSION", "2.1")

	os.MkdirAll(SettingsDir, 0755)

	tempDir := filepath.Join(SettingsDir, "Temp")
	os.MkdirAll(tempDir, 0755)

	// Set temp file path
	tempFilePath = filepath.Join(tempDir, "temp_autosave.dat")

	loadSettings()
	if appSettings.EchoVRDataPath == "" {
		appSettings.EchoVRDataPath = DefaultEchoPath
	}
	if appSettings.AssetsPath == "" || appSettings.AssetsPath == "./thumbnails" {
		appSettings.AssetsPath = filepath.Join(SettingsDir, "thumbnail")
	}
	saveSettings()

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("CRITICAL ERROR:", r)
			time.Sleep(10 * time.Second)
		}
	}()

	go downloadAndExtractThumbnails()

	a := app.New()
	a.SetIcon(fyne.NewStaticResource("icon.ico", embeddedIcon))

	w := a.NewWindow("Cosmetic Tint Editor")
	w.Resize(fyne.NewSize(1100, 800))

	statusLabel := widget.NewLabel("Status: Initializing...")

	nameEntry := widget.NewEntry()
	descEntry := widget.NewEntry()
	thumbIdEntry := widget.NewEntry()

	thumbImage := canvas.NewImageFromResource(nil)
	thumbImage.FillMode = canvas.ImageFillContain
	thumbImage.SetMinSize(fyne.NewSize(150, 150))
	thumbContainer := container.NewMax(
		canvas.NewRectangle(color.RGBA{30, 30, 30, 255}),
		thumbImage,
	)

	primaryHex := widget.NewEntry()
	primaryHex.PlaceHolder = "FFFFFF"
	secondaryHex := widget.NewEntry()
	secondaryHex.PlaceHolder = "FFFFFF"

	thumbIdEntry.Validator = func(s string) error {
		_, err := strconv.ParseInt(s, 10, 64)
		return err
	}
	hexValidator := func(s string) error {
		s = strings.TrimPrefix(s, "#")
		if len(s) != 6 {
			return fmt.Errorf("must be 6 chars")
		}
		if _, err := hex.DecodeString(s); err != nil {
			return fmt.Errorf("invalid hex")
		}
		return nil
	}
	primaryHex.Validator = hexValidator
	secondaryHex.Validator = hexValidator

	// --- FILE OPERATIONS ---
	saveToTemp := func() {
		b, err := cosmeticListToBytes(currentCList)
		if err != nil {
			fmt.Println("Error saving temp:", err)
			return
		}
		os.WriteFile(tempFilePath, b, 0644)
	}

	loadFromTemp := func() {
		if _, err := os.Stat(tempFilePath); os.IsNotExist(err) {
			dataCopy := make([]byte, len(embeddedOriginal))
			copy(dataCopy, embeddedOriginal)
			os.WriteFile(tempFilePath, dataCopy, 0644)
		}
		b, err := os.ReadFile(tempFilePath)
		if err != nil {
			statusLabel.SetText("Error reading temp file!")
			return
		}
		cList, err := bytesToCosmeticList(b)
		if err != nil {
			statusLabel.SetText("Error parsing temp file!")
			return
		}
		currentCList = cList
	}

	applyChange := func(modifier func(*CTint)) {
		if isLoadingEntry {
			return
		}
		if selectedListIndex == -1 {
			return
		}

		idx := selectedListIndex
		t := CTint{}
		if err := t.FromCosmeticEntry(currentCList.cosmeticEntries[idx]); err != nil {
			return
		}

		modifier(&t)

		newEntry, err := t.ToCosmeticEntry()
		if err != nil {
			return
		}

		// Fix Size Difference
		oldSize := len(currentCList.cosmeticEntries[idx].cEntryExtData)
		newSize := len(newEntry.cEntryExtData)
		currentCList.listSize = uint64(int64(currentCList.listSize) + int64(newSize - oldSize))
		currentCList.cosmeticEntries[idx] = newEntry

		saveToTemp()
	}

	onName := func(s string) { applyChange(func(t *CTint) { t.DisplayName = s }) }
	onDesc := func(s string) { applyChange(func(t *CTint) { t.Description = s }) }

	refreshThumbnail := func(idStr string) {
		if idStr == "" {
			return
		}
		if appSettings.AssetsPath != "" {
			originalPath := filepath.Join(appSettings.AssetsPath, idStr+".png")
			if _, err := os.Stat(originalPath); err == nil {
				thumbImage.File = originalPath
				thumbImage.Refresh()
				return
			}
		}

		thumbImage.File = ""
		thumbImage.Refresh()
	}

	onThumb := func(s string) {
		refreshThumbnail(s)
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			applyChange(func(t *CTint) { t.ThumbnailSymbol = id })
		}
	}

	onHexChange := func(isPrimary bool) func(string) {
		return func(s string) {
			s = strings.TrimPrefix(s, "#")
			if len(s) != 6 {
				return
			}
			bytes, err := hex.DecodeString(s)
			if err != nil {
				return
			}
			r, g, b := float32(bytes[0])/255.0, float32(bytes[1])/255.0, float32(bytes[2])/255.0

			applyChange(func(t *CTint) {
				if isPrimary {
					t.PrimaryColor_R, t.PrimaryColor_G, t.PrimaryColor_B = r, g, b
				} else {
					t.SecondaryColor_R, t.SecondaryColor_G, t.SecondaryColor_B = r, g, b
				}
			})
		}
	}

	unbindListeners := func() {
		nameEntry.OnChanged = nil
		descEntry.OnChanged = nil
		thumbIdEntry.OnChanged = nil
		primaryHex.OnChanged = nil
		secondaryHex.OnChanged = nil
	}

	bindListeners := func() {
		nameEntry.OnChanged = onName
		descEntry.OnChanged = onDesc
		thumbIdEntry.OnChanged = onThumb
		primaryHex.OnChanged = onHexChange(true)
		secondaryHex.OnChanged = onHexChange(false)
	}

	// --- THUMBNAIL GENERATOR ---
	generateAndSaveThumbnail := func() {
		// 1. Validate ID
		idStr := thumbIdEntry.Text
		if idStr == "" {
			dialog.ShowInformation("Error", "No Thumbnail ID (Symbol) set.", w)
			return
		}

		// 2. Load Embedded Template
		img, _, err := image.Decode(bytes.NewReader(embeddedTemplate))
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to decode embedded template: %v", err), w)
			return
		}

		// 3. Colours Helper
		parseColor := func(hexStr string) color.RGBA {
			hexStr = strings.TrimPrefix(hexStr, "#")
			b, _ := hex.DecodeString(hexStr)
			if len(b) < 3 {
				return color.RGBA{255, 255, 255, 255}
			}
			return color.RGBA{b[0], b[1], b[2], 255}
		}
		cPrim := parseColor(primaryHex.Text)
		cSec := parseColor(secondaryHex.Text)
		
		// Original Colorus in Template to Replace
		srcPrimary := color.RGBA{0x9F, 0x12, 0x13, 0xFF}
		srcSecondary := color.RGBA{0xEC, 0xDB, 0x10, 0xFF}

		isSimilar := func(c1, c2 color.RGBA, threshold float64) bool {
			rDiff := float64(c1.R) - float64(c2.R)
			gDiff := float64(c1.G) - float64(c2.G)
			bDiff := float64(c1.B) - float64(c2.B)
			return math.Sqrt(rDiff*rDiff+gDiff*gDiff+bDiff*bDiff) < threshold
		}

		bounds := img.Bounds()
		dst := image.NewRGBA(bounds)

		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				srcC := img.At(x, y)
				r, g, b, a := srcC.RGBA()
				// Convert back to 8-bit
				currColor := color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
				
				finalColor := currColor
				if isSimilar(currColor, srcPrimary, 80.0) {
					finalColor = color.RGBA{cPrim.R, cPrim.G, cPrim.B, currColor.A}
				} else if isSimilar(currColor, srcSecondary, 80.0) {
					finalColor = color.RGBA{cSec.R, cSec.G, cSec.B, currColor.A}
				}
				dst.Set(x, y, finalColor)
			}
		}

		// 5. Save DDS Texture
		// Path: Settings/input-pcvr/0/-4707359568332879775/[ID]
		texDir := filepath.Join(InputDir, ThumbTexFolder)
		if err := os.MkdirAll(texDir, 0755); err != nil {
			dialog.ShowError(fmt.Errorf("failed to create tex dir: %v", err), w)
			return
		}
		
		outDdsPath := filepath.Join(texDir, idStr)
		_, err = writeDDS(outDdsPath, dst)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to save DDS: %v", err), w)
			return
		}

		// 6. Save Metadata File
		// Path: Settings/input-pcvr/0/5353709876897953952/[ID]
		metaDir := filepath.Join(InputDir, ThumbMetaFolder)
		if err := os.MkdirAll(metaDir, 0755); err != nil {
			dialog.ShowError(fmt.Errorf("failed to create meta dir: %v", err), w)
			return
		}

		outMetaPath := filepath.Join(metaDir, idStr)
		if err := writeMetadata(outMetaPath); err != nil {
			dialog.ShowError(fmt.Errorf("failed to save metadata: %v", err), w)
			return
		}

		dialog.ShowInformation("Success", "Thumbnail files generated in input folder.\nID: "+idStr, w)
		statusLabel.SetText("Generated Thumbnail: " + idStr)
	}

	btnGenThumb := widget.NewButton("Generate & Save Thumbnail", generateAndSaveThumbnail)

	// --- REPACK / EXTRACT LOGIC ---
	var showRepackDialog func() 

	// Recursive Copy Helper
	copyRecursive := func(src, dst string) error {
		return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			// Compute relative path
			relPath, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			dstPath := filepath.Join(dst, relPath)

			if info.IsDir() {
				return os.MkdirAll(dstPath, info.Mode())
			}

			// Skip non-regular files (pipes, devices, etc.)
			if !info.Mode().IsRegular() {
				return nil
			}

			sourceFile, err := os.Open(path)
			if err != nil {
				return err
			}
			defer sourceFile.Close()

			destinationFile, err := os.Create(dstPath)
			if err != nil {
				return err
			}
			defer destinationFile.Close()

			_, err = io.Copy(destinationFile, sourceFile)
			return err
		})
	}

	runExtract := func(echoPath string) error {
		// Ensure parent folder exists
		os.MkdirAll(ExtractedDir, 0755)

		// Points to Settings/evrFileTools.exe
		toolPath := filepath.Join(SettingsDir, "evrFileTools.exe")

		cmd := exec.Command(toolPath,
			"-mode", "extract",
			"-packageName", PackageName,
			"-dataDir", echoPath,
			"-outputDir", ExtractedDir,
			"-tintsonly",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("extract failed: %v\nOutput: %s", err, string(out))
		}
		return nil
	}

	// Helper function for Part 1 of Repack: Execute Tool
	executeRepackTool := func(echoPath string) error {
		// 1. Prepare Input File for Tints
		// Path: Settings/input-pcvr/0/3671295590506143214/4869319423857648486
		tintDir := filepath.Join(InputDir, TintFolder)
		if err := os.MkdirAll(tintDir, 0755); err != nil {
			return fmt.Errorf("failed to create tint dir: %v", err)
		}

		outFile := filepath.Join(tintDir, TintFileName)
		b, err := cosmeticListToBytes(currentCList)
		if err != nil {
			return fmt.Errorf("failed to serialize tint data: %v", err)
		}
		if err := os.WriteFile(outFile, b, 0644); err != nil {
			return fmt.Errorf("failed to write tint file: %v", err)
		}

		// 2. Run evrFileTools Replace
		// Ensure output dir exists
		os.MkdirAll(OutputDir, 0755)

		// Points to Settings/evrFileTools.exe
		toolPath := filepath.Join(SettingsDir, "evrFileTools.exe")

		cmd := exec.Command(toolPath,
			"-mode", "replace",
			"-outputDir", OutputDir,
			"-packageName", PackageName,
			"-dataDir", echoPath,
			"-inputDir", InputDir,
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("repack failed: %v\nOutput: %s", err, string(out))
		}
		return nil
	}

	// Helper function for Part 2 of Repack: Push Files
	pushRepackedFiles := func(echoPath string) error {
		// 3. Move Files from OutputDir to Echo Data Dir
		// We can reuse copyRecursive since OutputDir structure mirrors EchoDataDir (packages/manifests)
		
		srcPkg := filepath.Join(OutputDir, "packages")
		dstPkg := filepath.Join(echoPath, "packages")
		if _, err := os.Stat(srcPkg); err == nil {
			if err := copyRecursive(srcPkg, dstPkg); err != nil {
				return fmt.Errorf("failed to move packages: %v", err)
			}
		}

		srcMan := filepath.Join(OutputDir, "manifests")
		dstMan := filepath.Join(echoPath, "manifests")
		if _, err := os.Stat(srcMan); err == nil {
			if err := copyRecursive(srcMan, dstMan); err != nil {
				return fmt.Errorf("failed to move manifests: %v", err)
			}
		}

		return nil
	}

	showRepackDialog = func() {
		// Container for the modal content
		content := container.NewVBox()
		modal := dialog.NewCustom("Repack Tool", "Close", content, w)
		modal.Resize(fyne.NewSize(600, 400))

		// Check if extracted exists
		_, errExtract := os.Stat(ExtractedDir)
		extractedExists := errExtract == nil

		// UI Elements
		lblPath := widget.NewLabel(appSettings.EchoVRDataPath)
		btnBrowse := widget.NewButton("Browse Data Path", func() {
			dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
				if uri != nil {
					appSettings.EchoVRDataPath = uri.Path()
					lblPath.SetText(appSettings.EchoVRDataPath)
					saveSettings()
				}
			}, w)
		})

		refreshUI := func() {
			content.Objects = nil // Clear

			if !extractedExists {
				// STATE 1: EXTRACT
				content.Add(widget.NewLabel("Step 1: Extract Original Tints"))
				content.Add(widget.NewLabel("Selected EchoVR Data Path:"))
				content.Add(container.NewBorder(nil, nil, nil, btnBrowse, lblPath))
				content.Add(widget.NewButton("Extract", func() {
					// Use a blocking dialog that runs extract in background to avoid freeze
					loading := dialog.NewCustom("Extracting...", "Cancel", widget.NewProgressBarInfinite(), w)
					loading.Show()
					
					go func() {
						err := runExtract(appSettings.EchoVRDataPath)
						loading.Hide()
						if err != nil {
							dialog.ShowError(err, w)
						} else {
							extractedExists = true
							// Refresh the modal content on Main Thread if needed, mostly safe in Fyne
							showRepackDialog()
						}
					}()
				}))
			} else {
				// STATE 2: REPACK
				content.Add(widget.NewLabel("Step 2: Modify & Repack"))

				// Backup Logic
				backupDir := filepath.Join(SettingsDir, BackupDirName)
				_, errBackup := os.Stat(backupDir)
				backupExists := errBackup == nil

				backupUI := container.NewVBox()
				if backupExists {
					backupUI.Add(widget.NewLabel("Backup found."))
					
					btnRevert := widget.NewButton("Revert to Backup", func() {
						loading := dialog.NewCustom("Restoring Backup...", "Cancel", widget.NewProgressBarInfinite(), w)
						loading.Show()
						go func() {
							// Copy from Settings/Backup -> EchoDataPath
							err := copyRecursive(backupDir, appSettings.EchoVRDataPath)
							loading.Hide()
							if err != nil {
								dialog.ShowError(err, w)
							} else {
								dialog.ShowInformation("Success", "Game files reverted to backup.", w)
							}
						}()
					})
					// Style warning
					btnRevert.Importance = widget.WarningImportance
					backupUI.Add(btnRevert)

				} else {
					backupUI.Add(widget.NewLabel("No backup found."))
					backupUI.Add(widget.NewButton("Create Backup", func() {
						loading := dialog.NewCustom("Backing up...", "Cancel", widget.NewProgressBarInfinite(), w)
						loading.Show()
						go func() {
							// Ensure backup dir exists
							os.MkdirAll(backupDir, 0755)
							// Copy from EchoDataPath -> Settings/Backup
							err := copyRecursive(appSettings.EchoVRDataPath, backupDir)
							loading.Hide()
							if err != nil {
								dialog.ShowError(err, w)
							} else {
								dialog.ShowInformation("Backup", "Backup created successfully.", w)
								// Refresh UI to show Revert button
								showRepackDialog()
							}
						}()
					}))
				}
				// Use Widget Card instead of Group
				content.Add(widget.NewCard("Backup", "", backupUI))

				content.Add(widget.NewSeparator())
				content.Add(widget.NewLabel("Ready to Repack changes into game."))
				content.Add(widget.NewButton("REPACK & APPLY", func() {
					// 1. Show Loading (Repacking)
					loading := dialog.NewCustom("Repacking...", "Cancel", widget.NewProgressBarInfinite(), w)
					loading.Show()
					
					// 2. Execute Tool in background
					go func() {
						err := executeRepackTool(appSettings.EchoVRDataPath)
						loading.Hide()
						
						if err != nil {
							dialog.ShowError(err, w)
							return
						}

						// 3. Prompt User to Push Files
						dialog.ShowConfirm("Push files?", "Repack complete. Do you want to push files to the game?", func(confirm bool) {
							if confirm {
								// 4. Show Loading (Pushing)
								pushLoading := dialog.NewCustom("Pushing files...", "Cancel", widget.NewProgressBarInfinite(), w)
								pushLoading.Show()

								// 5. Execute Push in background
								go func() {
									err := pushRepackedFiles(appSettings.EchoVRDataPath)
									pushLoading.Hide()
									if err != nil {
										dialog.ShowError(err, w)
									} else {
										dialog.ShowInformation("Success", "Tints repacked and applied to game!", w)
									}
								}()
							}
						}, w)
					}()
				}))
			}
			content.Refresh()
		}

		refreshUI()
		modal.Show()
	}

	btnRepack := widget.NewButton("REPACK", showRepackDialog)

	// --- LIST WIDGET ---
	loadEntryToEditor := func(realIdx int) {
		unbindListeners()
		isLoadingEntry = true

		loadFromTemp()

		selectedListIndex = realIdx
		t := CTint{}
		if err := t.FromCosmeticEntry(currentCList.cosmeticEntries[realIdx]); err != nil {
			isLoadingEntry = false
			bindListeners()
			return
		}

		nameEntry.SetText(t.DisplayName)
		descEntry.SetText(t.Description)

		toHex := func(r, g, b float32) string {
			ri, gi, bi := int(r*255), int(g*255), int(b*255)
			return fmt.Sprintf("%02X%02X%02X", ri, gi, bi)
		}
		primaryHex.SetText(toHex(t.PrimaryColor_R, t.PrimaryColor_G, t.PrimaryColor_B))
		secondaryHex.SetText(toHex(t.SecondaryColor_R, t.SecondaryColor_G, t.SecondaryColor_B))

		thumbIdEntry.SetText(strconv.FormatInt(t.ThumbnailSymbol, 10))

		refreshThumbnail(thumbIdEntry.Text)

		isLoadingEntry = false
		bindListeners()
	}

	searchEntry := widget.NewEntry()
	searchEntry.PlaceHolder = "Search..."

	tintList := widget.NewList(
		func() int { return len(filteredIndices) },
		func() fyne.CanvasObject { return widget.NewLabel("Template") },
		func(id widget.ListItemID, item fyne.CanvasObject) {
			realIndex := filteredIndices[id]
			entry := currentCList.cosmeticEntries[realIndex]
			dName := string(bytes.TrimRight(entry.cEntry.DisplayNameString[:], "\x00"))
			item.(*widget.Label).SetText(dName)
		},
	)
	tintList.OnSelected = func(id widget.ListItemID) { loadEntryToEditor(filteredIndices[id]) }

	refreshFilter := func() {
		txt := strings.ToLower(searchEntry.Text)
		filteredIndices = []int{}
		for _, idx := range tintIndices {
			dName := strings.ToLower(string(bytes.TrimRight(currentCList.cosmeticEntries[idx].cEntry.DisplayNameString[:], "\x00")))
			if txt == "" || strings.Contains(dName, txt) {
				filteredIndices = append(filteredIndices, idx)
			}
		}
		tintList.Refresh()
	}
	searchEntry.OnChanged = func(s string) { refreshFilter() }

	refreshIndices := func() {
		tintIndices = []int{}
		tintSymbol := int64(ToSymbol("tint"))
		for i, e := range currentCList.cosmeticEntries {
			if e.cEntry.CosmeticTypeSymbol == tintSymbol {
				tintIndices = append(tintIndices, i)
			}
		}
		refreshFilter()
	}

	initData := func() {
		dataCopy := make([]byte, len(embeddedOriginal))
		copy(dataCopy, embeddedOriginal)
		os.WriteFile(tempFilePath, dataCopy, 0644)
		loadFromTemp()
		defer func() { if r := recover(); r != nil {} }()
		refreshIndices()
		statusLabel.SetText(fmt.Sprintf("Loaded. Temp file created."))
	}

	// --- EXTERNAL FILE LOADING ---
	loadExternalFile := func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if reader == nil {
				return
			}
			defer reader.Close()
			data, err := io.ReadAll(reader)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if _, err := bytesToCosmeticList(data); err != nil {
				dialog.ShowError(fmt.Errorf("Invalid file format: %v", err), w)
				return
			}
			os.WriteFile(tempFilePath, data, 0644)
			loadFromTemp()
			refreshIndices()
			statusLabel.SetText("Loaded External: " + reader.URI().Name())
			dialog.ShowInformation("Success", "Loaded "+reader.URI().Name(), w)
		}, w)
	}

	// --- SETTINGS UI ---
	lblAssets := widget.NewLabel(appSettings.AssetsPath)
	lblTexOut := widget.NewLabel(appSettings.TextureOutPath)
	lblMetaOut := widget.NewLabel(appSettings.MetadataOutPath)
	lblDataPath := widget.NewLabel(appSettings.EchoVRDataPath)

	if appSettings.AssetsPath == "" {
		lblAssets.SetText("Not Set")
	}
	if appSettings.TextureOutPath == "" {
		lblTexOut.SetText("Not Set")
	}
	if appSettings.MetadataOutPath == "" {
		lblMetaOut.SetText("Not Set")
	}

	btnSetTexOut := widget.NewButton("Browse", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if uri != nil {
				appSettings.TextureOutPath = uri.Path()
				lblTexOut.SetText(appSettings.TextureOutPath)
				saveSettings()
			}
		}, w)
	})

	btnSetMetaOut := widget.NewButton("Browse", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if uri != nil {
				appSettings.MetadataOutPath = uri.Path()
				lblMetaOut.SetText(appSettings.MetadataOutPath)
				saveSettings()
			}
		}, w)
	})
	
	btnSetDataPath := widget.NewButton("Browse", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if uri != nil {
				appSettings.EchoVRDataPath = uri.Path()
				lblDataPath.SetText(appSettings.EchoVRDataPath)
				saveSettings()
			}
		}, w)
	})

	showSettings := func() {
		content := container.NewVBox(
			widget.NewLabel("Settings"), widget.NewSeparator(),
			widget.NewLabel("Thumbnails are downloaded automatically."),
			widget.NewSeparator(),
			widget.NewLabel("EchoVR Data Path:"), container.NewBorder(nil, nil, nil, btnSetDataPath, lblDataPath),
			widget.NewSeparator(),
			widget.NewLabel("Output Texture Folder (DDS):"), container.NewBorder(nil, nil, nil, btnSetTexOut, lblTexOut),
			widget.NewLabel("Output Metadata Folder (Hex):"), container.NewBorder(nil, nil, nil, btnSetMetaOut, lblMetaOut),
		)
		dialog.NewCustom("Settings", "Close", content, w).Show()
	}
	btnSettings := widget.NewButton("Settings", showSettings)
	btnLoadFile := widget.NewButton("Load File", loadExternalFile)

	saveBtn := widget.NewButton("SAVE DATA FILE (Cosmetics)", func() {
		b, err := cosmeticListToBytes(currentCList)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}

		// Ensure directories exist: Settings/input-pcvr/0/3671295590506143214
		tintDir := filepath.Join(InputDir, TintFolder)
		if err := os.MkdirAll(tintDir, 0755); err != nil {
			dialog.ShowError(err, w)
			return
		}

		// Write to: Settings/input-pcvr/0/3671295590506143214/4869319423857648486
		targetFile := filepath.Join(tintDir, TintFileName)
		
		if err := os.WriteFile(targetFile, b, 0644); err != nil {
			dialog.ShowError(err, w)
			return
		}

		dialog.ShowInformation("Saved", "Cosmetic List Saved to:\n"+targetFile, w)
	})

	// --- LAYOUT ---
	colorForm := widget.NewForm(
		widget.NewFormItem("Main", primaryHex),
		widget.NewFormItem("Secondary", secondaryHex),
	)

	// Modified Layout: Repack at Top
	topRow := container.NewVBox(
		container.NewGridWithColumns(1, btnRepack),
		container.NewGridWithColumns(2, btnSettings, btnLoadFile),
	)
	
	left := container.NewBorder(container.NewVBox(topRow, widget.NewSeparator(), searchEntry), nil, nil, nil, tintList)

	right := container.NewVBox(
		widget.NewLabel("Preview"),
		container.NewCenter(thumbContainer),
		btnGenThumb,
		widget.NewSeparator(),
		widget.NewLabel("Info"), nameEntry, descEntry, thumbIdEntry,
		widget.NewSeparator(),
		widget.NewLabel("Colors"), colorForm,
		layout.NewSpacer(), saveBtn, statusLabel,
	)

	w.SetContent(container.NewHSplit(left, container.NewPadded(container.NewVScroll(right))))

	go func() {
		time.Sleep(200 * time.Millisecond)
		if _, err := os.Stat(tempFilePath); err == nil {
			loadFromTemp()
			refreshIndices()
			statusLabel.SetText("Resumed from autosave.")
		} else {
			initData()
		}
		if selectedListIndex != -1 {
			refreshThumbnail(thumbIdEntry.Text)
		}
	}()

	w.ShowAndRun()
}

// --- THUMBNAIL DOWNLOADER ---
func downloadAndExtractThumbnails() {
	url := "https://api.github.com/repos/heisthecat31/EchoVR-Tint-Editor/releases/tags/Editor"
	zipName := filepath.Join(SettingsDir, "thumbnail.zip")
	targetDir := filepath.Join(SettingsDir, "thumbnail")

	// Ensure Settings dir exists
	os.MkdirAll(SettingsDir, 0755)

	// Check if already extracted
	if _, err := os.Stat(targetDir); err == nil {
		fmt.Println("Thumbnails already present.")
		return
	}

	fmt.Println("Fetching release info...")
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Failed to fetch release info:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Failed to fetch release info: HTTP %d\n", resp.StatusCode)
		return
	}

	var release struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadUrl string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		fmt.Println("Failed to decode release info:", err)
		return
	}

	downloadUrl := ""
	for _, asset := range release.Assets {
		if asset.Name == "thumbnail.zip" {
			downloadUrl = asset.BrowserDownloadUrl
			break
		}
	}

	if downloadUrl == "" {
		fmt.Println("thumbnail.zip not found in release assets")
		return
	}

	fmt.Println("Downloading thumbnails from:", downloadUrl)
	resp, err = http.Get(downloadUrl)
	if err != nil {
		fmt.Println("Failed to download thumbnails:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Failed to download: HTTP %d\n", resp.StatusCode)
		return
	}

	out, err := os.Create(zipName)
	if err != nil {
		fmt.Println("Failed to create zip file:", err)
		return
	}
	
	_, err = io.Copy(out, resp.Body)
	out.Close() // Close before reading
	if err != nil {
		fmt.Println("Failed to save zip:", err)
		return
	}

	// Extract
	r, err := zip.OpenReader(zipName)
	if err != nil {
		fmt.Println("Failed to open zip:", err)
		return
	}
	defer r.Close()

	os.MkdirAll(targetDir, 0755)

	for _, f := range r.File {
		fpath := filepath.Join(targetDir, f.Name)
		
		// Check for ZipSlip
		if !strings.HasPrefix(fpath, filepath.Clean(targetDir)+string(os.PathSeparator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			continue
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			continue
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
	}
	
	// Clean up zip
	os.Remove(zipName)
	fmt.Println("Thumbnails downloaded and extracted.")
}

// --- METADATA WRITER ---
func writeMetadata(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	// 1. Padding (FFs) - 192 bytes
	padding := bytes.Repeat([]byte{0xFF}, 192)
	f.Write(padding)

	// 2. Exact Static Tail Structure (64 bytes)
	tail := []byte{
		0x01, 0x00, 0x00, 0x00,
		0x80, 0x00, 0x00, 0x00,
		0x80, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x63, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x80, 0x00, 0x00, 0x00,
		0x80, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x00, 0x00,
		// Static values:
		0x80, 0x00, 0x01, 0x00,
		0x00, 0x40, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}

	f.Write(tail)
	return nil
}

// --- DDS WRITER (Uncompressed RGBA8) ---
func writeDDS(filename string, img image.Image) (uint32, error) {
	f, err := os.Create(filename)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	const (
		DDS_MAGIC        = 0x20534444
		DDS_HEADER_SIZE  = 124
		DDSD_CAPS        = 0x1
		DDSD_HEIGHT      = 0x2
		DDSD_WIDTH       = 0x4
		DDSD_PITCH       = 0x8
		DDSD_PIXELFORMAT = 0x1000
		DDPF_ALPHAPIXELS = 0x1
		DDPF_RGB         = 0x40
		DDSCAPS_TEXTURE  = 0x1000
	)

	bounds := img.Bounds()
	width := uint32(bounds.Dx())
	height := uint32(bounds.Dy())
	pitch := width * 4 // 4 bytes per pixel

	// Write Magic (4 bytes)
	binary.Write(f, binary.LittleEndian, uint32(DDS_MAGIC))

	// Write Header (124 bytes)
	binary.Write(f, binary.LittleEndian, uint32(DDS_HEADER_SIZE))
	binary.Write(f, binary.LittleEndian, uint32(DDSD_CAPS|DDSD_HEIGHT|DDSD_WIDTH|DDSD_PITCH|DDSD_PIXELFORMAT))
	binary.Write(f, binary.LittleEndian, height)
	binary.Write(f, binary.LittleEndian, width)
	binary.Write(f, binary.LittleEndian, pitch)
	binary.Write(f, binary.LittleEndian, uint32(0))
	binary.Write(f, binary.LittleEndian, uint32(0))
	for i := 0; i < 11; i++ {
		binary.Write(f, binary.LittleEndian, uint32(0))
	}

	// Pixel Format
	binary.Write(f, binary.LittleEndian, uint32(32))
	binary.Write(f, binary.LittleEndian, uint32(DDPF_RGB|DDPF_ALPHAPIXELS))
	binary.Write(f, binary.LittleEndian, uint32(0))
	binary.Write(f, binary.LittleEndian, uint32(32))
	binary.Write(f, binary.LittleEndian, uint32(0x00FF0000))
	binary.Write(f, binary.LittleEndian, uint32(0x0000FF00))
	binary.Write(f, binary.LittleEndian, uint32(0x000000FF))
	binary.Write(f, binary.LittleEndian, uint32(0xFF000000))

	// Caps
	binary.Write(f, binary.LittleEndian, uint32(DDSCAPS_TEXTURE))
	binary.Write(f, binary.LittleEndian, uint32(0))
	binary.Write(f, binary.LittleEndian, uint32(0))
	binary.Write(f, binary.LittleEndian, uint32(0))
	binary.Write(f, binary.LittleEndian, uint32(0))

	// Write Pixel Data (BGRA)
	drawImg := image.NewRGBA(bounds)
	draw.Draw(drawImg, bounds, img, bounds.Min, draw.Src)

	// Calculate Data Size
	dataSize := uint32(0)
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			r, g, b, a := drawImg.At(x, y).RGBA()
			binary.Write(f, binary.LittleEndian, uint8(b>>8))
			binary.Write(f, binary.LittleEndian, uint8(g>>8))
			binary.Write(f, binary.LittleEndian, uint8(r>>8))
			binary.Write(f, binary.LittleEndian, uint8(a>>8))
			dataSize += 4
		}
	}

	totalSize := 4 + 124 + dataSize // Magic + Header + Data
	return totalSize, nil
}

// --- STANDARD HELPERS ---
func loadSettings() {
	appSettings.TextureOutPath = ""
	appSettings.MetadataOutPath = ""
	file, err := os.ReadFile(settingsFile)
	if err == nil {
		json.Unmarshal(file, &appSettings)
	}
}
func saveSettings() {
	b, _ := json.MarshalIndent(appSettings, "", "  ")
	os.WriteFile(settingsFile, b, 0644)
}

type cosmeticList struct {
	_               [8]byte
	listSize        uint64
	_               [12]byte
	unk1            uint32
	_               [8]byte
	listCount       uint64
	listCount2      uint64
	cosmeticEntries []cosmeticEntry
}

func bytesToCosmeticList(b []byte) (cosmeticList, error) {
	var cList cosmeticList
	if len(b) < 56 {
		return cList, fmt.Errorf("file too small")
	}
	cList.listSize = binary.LittleEndian.Uint64(b[8:16])
	cList.unk1 = binary.LittleEndian.Uint32(b[28:32])
	cList.listCount = binary.LittleEndian.Uint64(b[40:48])
	cList.listCount2 = binary.LittleEndian.Uint64(b[48:56])
	headerSize := 56 + int(cList.listCount)*664
	if len(b) < headerSize {
		return cList, fmt.Errorf("header incomplete")
	}
	extData := b[headerSize:]
	cList.cosmeticEntries = make([]cosmeticEntry, cList.listCount)
	reader := bytes.NewReader(b[56:headerSize])
	for i := 0; i < int(cList.listCount); i++ {
		binary.Read(reader, binary.LittleEndian, &cList.cosmeticEntries[i].cEntry)
		sz := int(cList.cosmeticEntries[i].cEntry.ImageListingEntrySize + cList.cosmeticEntries[i].cEntry.OtherEntrySize)
		if sz > 0 && sz <= len(extData) {
			cList.cosmeticEntries[i].cEntryExtData = make([]byte, sz)
			copy(cList.cosmeticEntries[i].cEntryExtData, extData[:sz])
			extData = extData[sz:]
		}
	}
	return cList, nil
}
func cosmeticListToBytes(cList cosmeticList) ([]byte, error) {
	buf, _ := binary.Append(nil, binary.LittleEndian, [8]byte{})
	buf, _ = binary.Append(buf, binary.LittleEndian, cList.listSize)
	buf, _ = binary.Append(buf, binary.LittleEndian, [12]byte{})
	buf, _ = binary.Append(buf, binary.LittleEndian, cList.unk1)
	buf, _ = binary.Append(buf, binary.LittleEndian, [8]byte{})
	buf, _ = binary.Append(buf, binary.LittleEndian, cList.listCount)
	buf, _ = binary.Append(buf, binary.LittleEndian, cList.listCount2)
	for i := 0; i < len(cList.cosmeticEntries); i++ {
		buf, _ = binary.Append(buf, binary.LittleEndian, cList.cosmeticEntries[i].cEntry)
	}
	for i := 0; i < len(cList.cosmeticEntries); i++ {
		buf, _ = binary.Append(buf, binary.LittleEndian, cList.cosmeticEntries[i].cEntryExtData)
	}
	return buf, nil
}
