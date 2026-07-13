package boosters

import (
	"bytes"
	"encoding/json"
	"fmt"
	data "github.com/EchoTools/cosmetic-editor/Data"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"io"
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
		widget.NewButtonWithIcon("Replace Model from .blend", theme.UploadIcon(), func() {
			c.AssetSymbol12 = data.HexToSymbol(sym12Entry.Text)
			importCustomBooster(state, &c)
		}),
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

func importCustomBooster(state *data.AppState, existingBooster *data.CBooster) {
	blendPath, err := data.PickFile("3D Models (*.blend;*.glb)|*.blend;*.glb|All Files (*.*)|*.*")
	if err != nil || blendPath == "" {
		return
	}

	outDir := filepath.Join(data.GetSettingsDir(), "Temp", "ChassisBake")
	os.MkdirAll(outDir, 0755)

	scriptPath := filepath.Join(data.GetSettingsDir(), "Temp", "Scripts", "backend_chassis_builder.py")

	loading := dialog.NewCustom("Processing Custom Booster...", "Please wait, Blender is working...", widget.NewProgressBarInfinite(), state.Window)
	loading.Show()

	go func() {
		args := []string{scriptPath, blendPath, outDir, "--type", "booster"}
		if existingBooster != nil {
			args = append(args, "--mesh-hash", fmt.Sprintf("%d", uint64(existingBooster.AssetSymbol12)))
			
			// Base mesh for Booster is Sym12
			meshSym := existingBooster.AssetSymbol12
			hashHex := fmt.Sprintf("%016x", uint64(meshSym))
			baseMeshPath, err := state.FindBaseMesh(hashHex)
			if err == nil {
				args = append(args, "--base-mesh-3p", baseMeshPath)
			}
		}
		
		args = append(args, "--export-dir", filepath.Join(data.GetSettingsDir(), "input-pcvr"))
		addonZipPath := filepath.Join(data.GetSettingsDir(), "Temp", "Scripts", "evr_mesh_importer.zip")
		args = append(args, "--addon-zip", addonZipPath)
		
		cmd := exec.Command("python", args...)
		output, err := cmd.CombinedOutput()
		
		fyne.Do(func() {
			loading.Hide()
		})

		if err != nil {
			fyne.Do(func() {
				if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 99 {
					dialog.ShowError(fmt.Errorf("Blender is not installed or not found in PATH.\nPlease install Blender to replace this model."), state.Window)
				} else {
					dialog.ShowError(fmt.Errorf("Blender processing failed:\n%s\nOutput:\n%s", err.Error(), string(output)), state.Window)
				}
			})
			return
		}

		manifestPath := filepath.Join(outDir, "manifest.json")
		b, err := os.ReadFile(manifestPath)
		if err != nil {
			fyne.Do(func() { dialog.ShowError(fmt.Errorf("Failed to read manifest: %v", err), state.Window) })
			return
		}

		var mf Manifest
		if err := json.Unmarshal(b, &mf); err != nil {
			fyne.Do(func() { dialog.ShowError(fmt.Errorf("Failed to parse manifest: %v", err), state.Window) })
			return
		}

		// Inject into CosmeticList if it's a new Booster
		fyne.Do(func() {
			if existingBooster == nil {
				newEntry := data.CosmeticEntry{}
				newEntry.CEntry = data.NewCDescriptor()
				
				newEntry.CEntry.CosmeticTypeSymbol = int64(data.ToSymbol("booster"))
				intName := fmt.Sprintf("rwd_booster_custom_%s", mf.MeshHashHex[:8])
				newEntry.CEntry.InternalNameSymbol = int64(data.ToSymbol(intName))
				newEntry.CEntry.InternalNameSymbol2 = newEntry.CEntry.InternalNameSymbol
				copy(newEntry.CEntry.InternalNameString[:], []byte(intName))
				copy(newEntry.CEntry.DisplayNameString[:], []byte("Custom Booster"))
				newEntry.CEntry.RaritySymbol = data.HexToSymbol(data.RarityLegendary)
				
				newEntry.CEntry.AssetSymbol5 = -1
				newEntry.CEntry.AssetSymbol6 = -1
				newEntry.CEntry.AssetSymbol11 = -1
				newEntry.CEntry.AssetSymbol12 = int64(mf.AssetSymbol5)
	
				state.CosmeticList.CosmeticEntries = append(state.CosmeticList.CosmeticEntries, newEntry)
				state.CosmeticList.ListCount = uint64(len(state.CosmeticList.CosmeticEntries))
				state.CosmeticList.ListCount2 = state.CosmeticList.ListCount
				
				state.RefreshIndices()
				RefreshFilter(state, searchEntry.Text)
			} else {
				entry := &state.CosmeticList.CosmeticEntries[state.SelectedIndex]
				entry.CEntry.AssetSymbol12 = int64(mf.AssetSymbol5)
				LoadToEditor(state, state.SelectedIndex)
			}
			
			destGeoDirGPU := filepath.Join(data.GetSettingsDir(), "input-pcvr", "e642bfb1abcf76df") // CGMeshListResource GPU
			destGeoDirPri := filepath.Join(data.GetSettingsDir(), "input-pcvr", "4e426f88c1b5d7ac") // CGMeshListResource Primary
			destTexDir := filepath.Join(data.GetSettingsDir(), "input-pcvr", "4a4c32c49300b8a0")    // CTextureResource GPU
			
			os.MkdirAll(destGeoDirGPU, 0755)
			os.MkdirAll(destGeoDirPri, 0755)
			os.MkdirAll(destTexDir, 0755)
			
			// The python script now exports the GPU, Primary, and ALL texture DDS files directly 
			// to the input-pcvr directories above using the TextureReplacer from the addon!

			msg := "Custom Booster added!\nThe mesh and textures were copied to Settings/input-pcvr.\nSave your dat file and repack the game."
			if existingBooster != nil {
				msg = "Booster Model replaced!\nThe new mesh and textures were copied to Settings/input-pcvr to override the existing item.\nRepack your game."
			}
			dialog.ShowInformation("Success", msg, state.Window)
		})
	}()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
