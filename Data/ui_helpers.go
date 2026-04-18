package data

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/nfnt/resize"
)

// NewCard wraps a content object in a padded container with a background.
func NewCard(title string, content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(color.RGBA{R: 28, G: 28, B: 28, A: 255})
	bg.StrokeColor = color.RGBA{R: 44, G: 44, B: 44, A: 255}
	bg.StrokeWidth = 1

	header := canvas.NewText(strings.ToUpper(title), color.RGBA{R: 158, G: 158, B: 158, A: 255})
	header.TextSize = 10
	header.TextStyle = fyne.TextStyle{Bold: true}

	return container.NewStack(
		bg,
		container.NewPadded(
			container.NewBorder(
				container.NewVBox(header, widget.NewSeparator()),
				nil, nil, nil,
				content,
			),
		),
	)
}

// EmbeddedTemplate is a fallback for thumbnail generation.
var EmbeddedTemplate []byte

// ApplyTintToImage applies a primary and secondary color tint to a source image.
func ApplyTintToImage(src image.Image, pri, sec color.RGBA) image.Image {
	if src == nil {
		return nil
	}
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)

	pfR, pfG, pfB := float32(pri.R)/255.0, float32(pri.G)/255.0, float32(pri.B)/255.0
	sfR, sfG, sfB := float32(sec.R)/255.0, float32(sec.G)/255.0, float32(sec.B)/255.0

	var wg sync.WaitGroup
	numWorkers := runtime.NumCPU()
	if numWorkers < 1 {
		numWorkers = 1
	}
	height := bounds.Max.Y - bounds.Min.Y
	rowsPerWorker := (height + numWorkers - 1) / numWorkers

	for w := 0; w < numWorkers; w++ {
		startY := bounds.Min.Y + w*rowsPerWorker
		endY := startY + rowsPerWorker
		if endY > bounds.Max.Y {
			endY = bounds.Max.Y
		}
		if startY >= endY {
			break
		}

		wg.Add(1)
		go func(yStart, yEnd int) {
			defer wg.Done()
			for y := yStart; y < yEnd; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					r, _, b, a := src.At(x, y).RGBA()

					or := float32(r>>8) / 255.0
					ob := float32(b>>8) / 255.0
					oa := float32(a>>8) / 255.0

					resR := (pfR * ob) + (sfR * or)
					resG := (pfG * ob) + (sfG * or)
					resB := (pfB * ob) + (sfB * or)

					if resR > 1.0 {
						resR = 1.0
					}
					if resG > 1.0 {
						resG = 1.0
					}
					if resB > 1.0 {
						resB = 1.0
					}

					dst.Set(x, y, color.RGBA{
						R: uint8(resR * 255),
						G: uint8(resG * 255),
						B: uint8(resB * 255),
						A: uint8(oa * 255),
					})
				}
			}
		}(startY, endY)
	}
	wg.Wait()
	return dst
}

// RefreshAssetPreview updates the original and replacement image widgets, optionally applying a tint.
func RefreshAssetPreview(state *AppState, origImg, replImg *canvas.Image, origPath, replPath string, allowTint bool) {
	if origImg == nil || replImg == nil {
		return
	}

	update := func(imgObj *canvas.Image, path string) {
		if path == "" {
			imgObj.Resource = nil
			imgObj.Image = nil
			imgObj.Refresh()
			return
		}

		// Core transformation logic
		process := func(p string) (fyne.Resource, image.Image) {
			f, err := os.Open(p)
			if err != nil {
				return nil, nil
			}
			defer f.Close()

			src, _, err := image.Decode(f)
			if err != nil {
				// Try loading as resource if decode fails (might be a resource)
				res, _ := fyne.LoadResourceFromPath(p)
				return res, nil
			}

			if allowTint && state.PreviewTintCheck != nil && state.PreviewTintCheck.Checked && state.PreviewTintIndex != -1 {
				tintIndices := state.CategoryIndices["Tints"]
				if state.PreviewTintIndex < len(tintIndices) {
					realTintIdx := tintIndices[state.PreviewTintIndex]
					entry := state.CosmeticList.CosmeticEntries[realTintIdx]

					// Extraction logic for Tint (6 floats, 24 bytes)
					extData := entry.CEntryExtData
					if len(extData) >= 24 {
						priR := *(*float32)(unsafe.Pointer(&extData[0]))
						priG := *(*float32)(unsafe.Pointer(&extData[4]))
						priB := *(*float32)(unsafe.Pointer(&extData[8]))
						secR := *(*float32)(unsafe.Pointer(&extData[12]))
						secG := *(*float32)(unsafe.Pointer(&extData[16]))
						secB := *(*float32)(unsafe.Pointer(&extData[20]))

						pri := color.RGBA{uint8(priR * 255), uint8(priG * 255), uint8(priB * 255), 255}
						sec := color.RGBA{uint8(secR * 255), uint8(secG * 255), uint8(secB * 255), 255}

						tinted := ApplyTintToImage(src, pri, sec)
						return nil, tinted
					}
				}
			}

			res, _ := fyne.LoadResourceFromPath(p)
			return res, nil
		}

		res, img := process(path)
		imgObj.Resource = res
		imgObj.Image = img
		imgObj.Refresh()
	}

	update(origImg, origPath)
	update(replImg, replPath)
}

// TrimZero removes trailing null bytes from a byte slice and returns it as a string.
func TrimZero(b []byte) string {
	return string(bytes.TrimRight(b, "\x00"))
}

// ParseFloat32 parses a string into a float32, returning 0 on error.
func ParseFloat32(s string) (float32, error) {
	val, err := strconv.ParseFloat(s, 32)
	return float32(val), err
}

// FindTool looks for tools in ./Settings OR [ExeDir]/Settings
func FindTool(settingsDir, toolName string) (string, error) {
	// 1. Check Working Directory (./Settings/tool.exe)
	localPath := filepath.Join(settingsDir, toolName)
	if _, err := os.Stat(localPath); err == nil {
		path, _ := filepath.Abs(localPath)
		return path, nil
	}

	// 2. Check Executable Directory ([ExeDir]/Settings/tool.exe)
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		exeToolPath := filepath.Join(exeDir, "settings", toolName)
		if _, err := os.Stat(exeToolPath); err == nil {
			return exeToolPath, nil
		}
	}

	return "", fmt.Errorf("tool '%s' not found", toolName)
}

// GetSettingsDir returns the absolute path to the Settings folder
func GetSettingsDir() string {
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	if strings.Contains(strings.ToLower(exeDir), "go-build") || strings.Contains(strings.ToLower(exeDir), "temp") {
		cwd, _ := os.Getwd()
		exeDir = cwd
	}

	abs, _ := filepath.Abs(filepath.Join(exeDir, "settings"))
	return abs
}

// GetDefaultEchoVRPath returns the first existing path from a set of common EchoVR installation locations.
func GetDefaultEchoVRPath() string {
	paths := []string{
		`C:\Oculus\Games\Software\Software\ready-at-dawn-echo-arena\_data\5932408047\rad15\win10`,
		`C:\EchoVR\ready-at-dawn-echo-arena\_data\5932408047\rad15\win10`,
		`C:\Program Files\Oculus\Software\Software\ready-at-dawn-echo-arena\_data\5932408047\rad15\win10`,
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// FindExtractedAsset attempts to find an asset file in the extracted folder base using various hex string formats.
func FindExtractedAsset(extBase string, symbol int64, folderName string) string {
	if symbol == -1 || extBase == "" {
		return ""
	}

	hexPadded := fmt.Sprintf("%016x", uint64(symbol))
	hexMinimal := fmt.Sprintf("%x", uint64(symbol))

	prefixes := []string{"", "0x"}
	extensions := []string{"", ".bin", ".dds"}
	hexVariants := []string{hexMinimal, hexPadded}

	folderPath := filepath.Join(extBase, folderName)

	for _, hexV := range hexVariants {
		for _, pre := range prefixes {
			for _, ext := range extensions {
				p := filepath.Join(folderPath, pre+hexV+ext)
				if _, err := os.Stat(p); err == nil {
					return p
				}
			}
		}
	}

	// Fallback to root of extracted if not found in folder
	for _, hexV := range hexVariants {
		for _, pre := range prefixes {
			for _, ext := range extensions {
				p := filepath.Join(extBase, pre+hexV+ext)
				if _, err := os.Stat(p); err == nil {
					return p
				}
			}
		}
	}

	return ""
}

// EnsureTextureCached checks if a PNG exists in the cache and attempts to create it from extracted files if missing.
func EnsureTextureCached(state *AppState, hexStr string) {
	if hexStr == "" || hexStr == "0" {
		return
	}

	settingsPath := GetSettingsDir()
	if state.Settings.TextureCachePath == "" {
		state.Settings.TextureCachePath = filepath.Join(settingsPath, "texture_cache")
	}

	cacheDir, _ := filepath.Abs(state.Settings.TextureCachePath)
	os.MkdirAll(cacheDir, 0755)

	cachePath := filepath.Join(cacheDir, hexStr+".png")
	if _, err := os.Stat(cachePath); err == nil {
		return
	}

	// Not in cache, search in extracted AND input folders
	sourceFolders := []string{ThumbTexFolderPC, ThumbMetaFolderPC, TexTexFolderPC, TexMetaFolderPC}
	inputBase := filepath.Join(settingsPath, InputDirNamePC)
	if state.Settings.Mode == "Quest" {
		sourceFolders = append([]string{ThumbTexFolderQuest, ThumbMetaFolderQuest, TexTexFolderQuest, TexMetaFolderQuest}, sourceFolders...)
		inputBase = filepath.Join(settingsPath, InputDirNameQuest)
	}

	var extractedPath string
	symVal := HexToSymbol(hexStr)

	// 1. Try extracted folders
	extPath := state.Settings.ExtractedPath
	if extPath == "" {
		extPath = filepath.Join(settingsPath, ExtractedDirName)
	}

	for _, folder := range sourceFolders {
		extractedPath = FindExtractedAsset(extPath, symVal, folder)
		if extractedPath != "" {
			break
		}
	}

	// 2. Try input folders (newly repacked assets)
	if extractedPath == "" {
		for _, folder := range sourceFolders {
			extractedPath = FindExtractedAsset(inputBase, symVal, folder)
			if extractedPath != "" {
				fmt.Printf("[Cache] Found repacked asset for %s at: %s\n", hexStr, extractedPath)
				break
			}
		}
	}

	if extractedPath == "" {
		// Log failures for frame 0 to help debug
		if strings.HasSuffix(hexStr, "0") || len(hexStr) > 14 {
			absInput, _ := filepath.Abs(inputBase)
			fmt.Printf("[Cache] Asset %s not found. Searched:\n  - %s\n  - %s\n", hexStr, extPath, absInput)
		}
		return
	}

	if state.StatusLabel != nil {
		state.StatusLabel.SetText("Caching: " + hexStr + "...")
	}

	// Found original, convert to PNG
	texconvPath, err := FindTool(settingsPath, "texconv.exe")
	if err != nil {
		if state.StatusLabel != nil {
			state.StatusLabel.SetText("texconv not found")
		}
		return
	}

	tempDir := filepath.Join(settingsPath, "Temp")
	os.MkdirAll(tempDir, 0755)
	tempDds := filepath.Join(tempDir, hexStr+".dds")

	// Copy to temp
	srcData, err := os.ReadFile(extractedPath)
	if err != nil {
		return
	}
	if err := os.WriteFile(tempDds, srcData, 0644); err != nil {
		return
	}

	// Convert: keeps name but adds .png
	outPng := filepath.Join(cacheDir, hexStr+".png")
	cmd := exec.Command(texconvPath, "decode", tempDds, outPng)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := cmd.CombinedOutput(); err != nil {
		if state.StatusLabel != nil {
			state.StatusLabel.SetText("texconv failed for " + hexStr)
		}
		fmt.Printf("texconv error: %v\nOutput: %s\n", err, string(out))
	} else {
		if state.StatusLabel != nil {
			state.StatusLabel.SetText("Cached: " + hexStr)
		}
	}

	// Cleanup
	os.Remove(tempDds)
}

const (
	PackageName = "48037dc70b0ecab2"

	// Base Directories
	ExtractedDirName = "pcvr-extracted"
	OutputDirName    = "output-both"
	BackupDirName    = "Backup"

	// PCVR Constants
	InputDirNamePC    = "input-pcvr"
	TintFolderPC      = "32f30fe361939dee"
	TintFileNamePC    = "43934c379cf1e366"
	ThumbTexFolderPC  = "beac1969cb7b8861"
	ThumbMetaFolderPC = "4a4c32c49300b8a0"
	TexTexFolderPC    = "beac1969cb7b8861"
	TexMetaFolderPC   = "4a4c32c49300b8a0"

	// Quest Constants
	InputDirNameQuest    = "input-quest"
	TintFolderQuest      = "24cbfd54e9a7f2ea"
	TintFileNameQuest    = "bb758e9809650e3b"
	ThumbTexFolderQuest  = "489bb35d53ca50e9"
	ThumbMetaFolderQuest = "e2ef0854d0cd69b8"
	TexTexFolderQuest    = "489bb35d53ca50e9"
	TexMetaFolderQuest   = "e2ef0854d0cd69b8"
)

// HandlePNGThumbnailReplacement allows direct replacement of a thumbnail with a PNG file.
func HandlePNGThumbnailReplacement(state *AppState, symbol string, selectedPngPath string, btn *widget.Button) {
	if symbol == "" || selectedPngPath == "" {
		return
	}
	if btn != nil {
		btn.Disable()
	}
	state.StatusLabel.SetText("Replacing thumbnail with PNG...")
	w := state.Window

	go func() {
		defer fyne.Do(func() {
			if btn != nil {
				btn.Enable()
			}
		})

		settingsPath := GetSettingsDir()
		tempDir := filepath.Join(settingsPath, "Temp")
		os.MkdirAll(tempDir, 0755)

		// 1. Convert PNG to Correct Texture Format
		var generatedFile string
		if state.Settings.Mode == "Quest" {
			astcPath, err := FindTool(settingsPath, "astcenc-avx2.exe")
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, w) })
				return
			}
			tempAstcPath := filepath.Join(tempDir, "temp_thumb.astc")
			cmd := exec.Command(astcPath, "-cs", selectedPngPath, tempAstcPath, "6x6", "-medium")
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			if out, err := cmd.CombinedOutput(); err != nil {
				fyne.Do(func() { dialog.ShowError(fmt.Errorf("astcenc failed: %s", out), w) })
				return
			}
			astcData, _ := os.ReadFile(tempAstcPath)
			if len(astcData) < 16 {
				fyne.Do(func() { dialog.ShowError(fmt.Errorf("failed to generate ASTC"), w) })
				return
			}
			header := make([]byte, 16)
			header[0], header[1], header[2], header[3] = 0x53, 0x80, 0x09, 0x00
			binary.LittleEndian.PutUint16(header[4:], 6)
			binary.LittleEndian.PutUint16(header[6:], 6)
			// Rough estimate of size for header
			header[8], header[9] = 0x00, 0x01 // 256
			header[10], header[11] = 0x00, 0x01
			finalData := append(header, astcData[16:]...)
			generatedFile = filepath.Join(tempDir, "temp_thumb_stripped.bin")
			os.WriteFile(generatedFile, finalData, 0644)
		} else {
			texconvPath, err := FindTool(settingsPath, "texconv.exe")
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, w) })
				return
			}
			generatedFile = filepath.Join(tempDir, "temp_thumb.dds")
			cmd := exec.Command(texconvPath, "encode", selectedPngPath, generatedFile)
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			if out, err := cmd.CombinedOutput(); err != nil {
				fyne.Do(func() { dialog.ShowError(fmt.Errorf("texconv failed: %s", out), w) })
				return
			}
		}

		// 2. Repack
		inputDir := InputDirNamePC
		metaFolder := ThumbMetaFolderPC
		texFolder := ThumbTexFolderPC
		if state.Settings.Mode == "Quest" {
			inputDir = InputDirNameQuest
			metaFolder = ThumbMetaFolderQuest
			texFolder = ThumbTexFolderQuest
		}

		absInputDir := filepath.Join(settingsPath, inputDir)
		texDir := filepath.Join(absInputDir, texFolder)
		os.MkdirAll(texDir, 0755)
		targetTex := filepath.Join(texDir, symbol)
		os.Rename(generatedFile, targetTex)

		metaDir := filepath.Join(absInputDir, metaFolder)
		os.MkdirAll(metaDir, 0755)
		fi, _ := os.Stat(targetTex)

		symVal := HexToSymbol(symbol)
		extPath := state.Settings.ExtractedPath
		if extPath == "" {
			extPath = filepath.Join(settingsPath, ExtractedDirName)
		}
		srcMetaPath := FindExtractedAsset(extPath, symVal, metaFolder)

		WriteMetadata(filepath.Join(metaDir, symbol), state.Settings.Mode, srcMetaPath, uint32(fi.Size()))

		fyne.Do(func() {
			state.StatusLabel.SetText("Successfully replaced fanfare thumbnail.")
			dialog.ShowInformation("Success", "Fanfare thumbnail replaced with PNG.", w)
			state.UpdateSidebarThumbnail(HexToSymbol(symbol))
		})
	}()
}

// WriteMetadata generates or patches a metadata file for textures.
func WriteMetadata(filename, mode string, srcPath string, newDdsSize uint32) error {
	// If srcPath is provided, we PATCH the original metadata
	if srcPath != "" {
		if _, err := os.Stat(srcPath); err == nil {
			data, err := os.ReadFile(srcPath)
			if err == nil && len(data) >= 256 {
				// Patch DDS Size at 244:248
				binary.LittleEndian.PutUint32(data[244:248], newDdsSize)
				return os.WriteFile(filename, data, 0644)
			}
		}
	}

	// Fallback to generating a generic one if patching fails or no srcPath
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	padding := bytes.Repeat([]byte{0xFF}, 192)
	f.Write(padding)

	var tail []byte
	if mode == "Quest" {
		tail = []byte{
			0x01, 0x00, 0x00, 0x00, 0x80, 0x00, 0x00, 0x00,
			0x80, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
			0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x63, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x80, 0x00, 0x00, 0x00, 0x80, 0x00, 0x00, 0x00,
			0x01, 0x00, 0x00, 0x00, 0x40, 0x1E, 0x00, 0x00,
			0x00, 0x40, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		}
	} else {
		tail = []byte{
			0x01, 0x00, 0x00, 0x00, 0x80, 0x00, 0x00, 0x00,
			0x80, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
			0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x63, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x80, 0x00, 0x00, 0x00, 0x80, 0x00, 0x00, 0x00,
			0x01, 0x00, 0x00, 0x00, 0x04, 0x56, 0x00, 0x00,
			0x00, 0x40, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		}
	}

	binary.LittleEndian.PutUint32(tail[52:56], newDdsSize) // New size field in tail too? Original main.go did tail[52:56] in some branches
	f.Write(tail)
	return nil
}

// GenerateAndSaveThumbnail generates a tinted thumbnail and saves it to the input directory.
func GenerateAndSaveThumbnail(state *AppState, primHexTxt, secHexTxt, idStr string) {
	if idStr == "" {
		dialog.ShowInformation("Error", "No Thumbnail ID (Symbol) set.", state.Window)
		return
	}

	idStr = strings.ToLower(idStr)
	mode := state.Settings.Mode
	w := state.Window

	loading := dialog.NewCustom("Generating...", "Cancel", widget.NewProgressBarInfinite(), w)
	loading.Show()

	go func() {
		defer loading.Hide()

		if len(EmbeddedTemplate) == 0 {
			dialog.ShowError(fmt.Errorf("template_thumb.png not found! Please place it in the root directory."), w)
			return
		}

		img, _, err := image.Decode(bytes.NewReader(EmbeddedTemplate))
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to decode template: %v", err), w)
			return
		}

		parseColor := func(hexStr string) color.RGBA {
			hexStr = strings.TrimPrefix(hexStr, "#")
			b, _ := hex.DecodeString(hexStr)
			if len(b) < 3 {
				return color.RGBA{255, 255, 255, 255}
			}
			return color.RGBA{b[0], b[1], b[2], 255}
		}
		cPrim := parseColor(primHexTxt)
		cSec := parseColor(secHexTxt)

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
				currColor := color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}

				finalColor := currColor
				wasReplaced := false
				if isSimilar(currColor, srcPrimary, 20.0) {
					finalColor = color.RGBA{cPrim.R, cPrim.G, cPrim.B, currColor.A}
					wasReplaced = true
				} else if isSimilar(currColor, srcSecondary, 20.0) {
					finalColor = color.RGBA{cSec.R, cSec.G, cSec.B, currColor.A}
					wasReplaced = true
				}

				if wasReplaced {
					finalColor.R = uint8(float32(finalColor.R) * 0.6)
					finalColor.G = uint8(float32(finalColor.G) * 0.6)
					finalColor.B = uint8(float32(finalColor.B) * 0.6)
				}
				dst.Set(x, y, finalColor)
			}
		}

		settingsPath := GetSettingsDir()
		tempDir := filepath.Join(settingsPath, "Temp")
		os.MkdirAll(tempDir, 0755)

		tempPngPath := filepath.Join(tempDir, "temp_thumb.png")
		fPng, _ := os.Create(tempPngPath)
		png.Encode(fPng, dst)
		fPng.Close()

		var generatedFile string
		if mode == "Quest" {
			astcPath, err := FindTool(settingsPath, "astcenc-avx2.exe")
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			tempAstcPath := filepath.Join(tempDir, "temp_thumb.astc")
			cmd := exec.Command(astcPath, "-cs", tempPngPath, tempAstcPath, "6x6", "-medium")
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			if out, err := cmd.CombinedOutput(); err != nil {
				dialog.ShowError(fmt.Errorf("astcenc failed: %s", out), w)
				return
			}

			astcData, _ := os.ReadFile(tempAstcPath)
			header := make([]byte, 16)
			header[0], header[1], header[2], header[3] = 0x53, 0x80, 0x09, 0x00
			binary.LittleEndian.PutUint16(header[4:], 6)
			binary.LittleEndian.PutUint16(header[6:], 6)
			binary.LittleEndian.PutUint16(header[8:], uint16(bounds.Dx()))
			binary.LittleEndian.PutUint16(header[10:], uint16(bounds.Dy()))
			finalData := append(header, astcData[16:]...)
			generatedFile = filepath.Join(tempDir, "temp_thumb_stripped.bin")
			os.WriteFile(generatedFile, finalData, 0644)
		} else {
			texconvPath, err := FindTool(settingsPath, "texconv.exe")
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			generatedFile = filepath.Join(tempDir, "temp_thumb.dds")
			cmd := exec.Command(texconvPath, "encode", tempPngPath, generatedFile)
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
			if out, err := cmd.CombinedOutput(); err != nil {
				dialog.ShowError(fmt.Errorf("texconv failed: %s", out), w)
				return
			}
		}

		absInputDir := filepath.Join(settingsPath, "input-pcvr")
		if state.Settings.Mode == "Quest" {
			absInputDir = filepath.Join(settingsPath, "input-quest")
		}

		thumbTexFolder := "beac1969cb7b8861"
		thumbMetaFolder := "4a4c32c49300b8a0"
		if state.Settings.Mode == "Quest" {
			thumbTexFolder = "489bb35d53ca50e9"
			thumbMetaFolder = "e2ef0854d0cd69b8"
		}

		texDir := filepath.Join(absInputDir, thumbTexFolder)
		os.MkdirAll(texDir, 0755)
		targetTex := filepath.Join(texDir, idStr)
		os.Rename(generatedFile, targetTex)

		metaDir := filepath.Join(absInputDir, thumbMetaFolder)
		os.MkdirAll(metaDir, 0755)

		fi, _ := os.Stat(targetTex)
		WriteMetadata(filepath.Join(metaDir, idStr), mode, "", uint32(fi.Size()))

		os.Remove(tempPngPath)
		dialog.ShowInformation("Success", "Thumbnail generated: "+idStr, w)
	}()
}

func HandleTextureReplacement(state *AppState, symbol string, selectedPngPath string, btn *widget.Button, statusMsg string) {
	if symbol == "" || selectedPngPath == "" {
		return
	}
	btn.Disable()
	state.StatusLabel.SetText(statusMsg)
	w := state.Window

	go func() {
		defer fyne.Do(func() { btn.Enable() })

		settingsPath := GetSettingsDir()
		tempDir := filepath.Join(settingsPath, "Temp")
		os.MkdirAll(tempDir, 0755)

		texconvPath, err := FindTool(settingsPath, "texconv.exe")
		if err != nil {
			fyne.Do(func() { dialog.ShowError(err, w) })
			return
		}

		// Mode-based folders
		inputDir := InputDirNamePC
		metaFolder := TexMetaFolderPC
		texFolder := TexTexFolderPC
		if state.Settings.Mode == "Quest" {
			inputDir = InputDirNameQuest
			metaFolder = TexMetaFolderQuest
			texFolder = TexTexFolderQuest
		}

		// 1. Encode PNG to DDS (PCVR)
		finalPngPath := selectedPngPath

		// Resizing Logic for Parity
		if state.Settings.TextureCachePath != "" {
			origCachePath := filepath.Join(state.Settings.TextureCachePath, symbol+".png")
			if _, err := os.Stat(origCachePath); err == nil {
				fOrig, _ := os.Open(origCachePath)
				imgOrig, _, _ := image.Decode(fOrig)
				fOrig.Close()

				fRepl, _ := os.Open(selectedPngPath)
				imgRepl, _, _ := image.Decode(fRepl)
				fRepl.Close()

				if imgOrig != nil && imgRepl != nil {
					oB := imgOrig.Bounds()
					rB := imgRepl.Bounds()
					if oB.Dx() != rB.Dx() || oB.Dy() != rB.Dy() {
						resized := resize.Resize(uint(oB.Dx()), uint(oB.Dy()), imgRepl, resize.Lanczos3)
						finalPngPath = filepath.Join(tempDir, "resized_"+symbol+".png")
						fOut, _ := os.Create(finalPngPath)
						png.Encode(fOut, resized)
						fOut.Close()
						state.StatusLabel.SetText(fmt.Sprintf("Resized replacement to %dx%d", oB.Dx(), oB.Dy()))
					}
				}
			}
		}

		generatedFile := filepath.Join(tempDir, "temp_replacement.dds")
		cmd := exec.Command(texconvPath, "encode", finalPngPath, generatedFile)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if out, err := cmd.CombinedOutput(); err != nil {
			fyne.Do(func() { dialog.ShowError(fmt.Errorf("texconv failed: %s", out), w) })
			return
		}

		ddsData, err := os.ReadFile(generatedFile)
		if err != nil {
			fyne.Do(func() { dialog.ShowError(err, w) })
			return
		}
		ddsSize := uint32(len(ddsData))

		// 2. Resolve Original Metadata for Patching
		symVal := HexToSymbol(symbol)
		extPath := state.Settings.ExtractedPath
		if extPath == "" {
			extPath = filepath.Join(settingsPath, ExtractedDirName)
		}
		srcMetaPath := FindExtractedAsset(extPath, symVal, metaFolder)

		// 3. Save Patched Metadata to Input
		metaOutDir := filepath.Join(settingsPath, inputDir, metaFolder)
		os.MkdirAll(metaOutDir, 0755)
		metaOutPath := filepath.Join(metaOutDir, symbol)
		if err := WriteMetadata(metaOutPath, state.Settings.Mode, srcMetaPath, ddsSize); err != nil {
			fyne.Do(func() { dialog.ShowError(err, w) })
			return
		}

		// 4. Save Raw Texture to Input
		texOutDir := filepath.Join(settingsPath, inputDir, texFolder)
		os.MkdirAll(texOutDir, 0755)
		texOutPath := filepath.Join(texOutDir, symbol)
		if err := os.WriteFile(texOutPath, ddsData, 0644); err != nil {
			fyne.Do(func() { dialog.ShowError(err, w) })
			return
		}

		fyne.Do(func() {
			state.StatusLabel.SetText("Successfully replaced texture: " + symbol)
			dialog.ShowInformation("Success", "Texture and metadata updated for "+symbol, w)
		})
	}()
}

// PickFolder opens a native Windows folder selection dialog using PowerShell.
func PickFolder() (string, error) {
	const script = `
Add-Type -AssemblyName System.Windows.Forms
$f = New-Object System.Windows.Forms.OpenFileDialog
$f.ValidateNames = $false
$f.CheckFileExists = $false
$f.CheckPathExists = $true
$f.FileName = "Folder Selection."
if ($f.ShowDialog() -eq 'OK') { Split-Path $f.FileName }
`
	cmd := exec.Command("powershell", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// PickFile opens a native Windows file selection dialog using PowerShell.
func PickFile(fallbackFilter string) (string, error) {
	filter := fallbackFilter
	if filter == "" {
		filter = "All Files (*.*)|*.*"
	}
	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$f = New-Object Windows.Forms.OpenFileDialog
$f.Filter = '%s'
if ($f.ShowDialog() -eq 'OK') { $f.FileName }
`, filter)
	cmd := exec.Command("powershell", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// PickSaveFile opens a native Windows save file dialog using PowerShell.
func PickSaveFile(fallbackFilter, defaultName string) (string, error) {
	filter := fallbackFilter
	if filter == "" {
		filter = "GIF Files (*.gif)|*.gif|All Files (*.*)|*.*"
	}
	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$f = New-Object Windows.Forms.SaveFileDialog
$f.Filter = '%s'
$f.FileName = '%s'
if ($f.ShowDialog() -eq 'OK') { $f.FileName }
`, filter, defaultName)
	cmd := exec.Command("powershell", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
