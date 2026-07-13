package chassis

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
		widget.NewButtonWithIcon("Replace Model from .blend", theme.UploadIcon(), func() {
			c.AssetSymbol6 = data.HexToSymbol(sym6Entry.Text)
			c.AssetSymbol12 = data.HexToSymbol(sym12Entry.Text)
			importCustomChassis(state, &c)
		}),
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

func importCustomChassis(state *data.AppState, existingChassis *data.CChassis) {
	if existingChassis != nil {
		var dlg dialog.Dialog
		tpBtn := widget.NewButton("3rd Person", func() {
			dlg.Hide()
			doImportChassis(state, existingChassis, "3p")
		})
		fpBtn := widget.NewButton("1st Person", func() {
			dlg.Hide()
			doImportChassis(state, existingChassis, "1p")
		})
		bothBtn := widget.NewButton("Both", func() {
			dlg.Hide()
			doImportChassis(state, existingChassis, "both")
		})
		dlg = dialog.NewCustom("Select Variant to Replace", "Cancel", container.NewVBox(tpBtn, fpBtn, bothBtn), state.Window)
		dlg.Show()
	} else {
		doImportChassis(state, existingChassis, "both")
	}
}

func doImportChassis(state *data.AppState, existingChassis *data.CChassis, variant string) {
	blendPath, err := data.PickFile("3D Models (*.blend;*.glb)|*.blend;*.glb|All Files (*.*)|*.*")
	if err != nil || blendPath == "" {
		return
	}

	outDir := filepath.Join(data.GetSettingsDir(), "Temp", "ChassisBake")
	os.MkdirAll(outDir, 0755)

	scriptPath := filepath.Join(data.GetSettingsDir(), "Temp", "Scripts", "backend_chassis_builder.py")

	loading := dialog.NewCustom("Processing Custom Chassis...", "Please wait, Blender is working...", widget.NewProgressBarInfinite(), state.Window)
	loading.Show()

	go func() {
		args := []string{scriptPath, blendPath, outDir, "--type", "chassis", "--target", variant}
		if existingChassis != nil {
			args = append(args, "--mesh-hash", fmt.Sprintf("%d", uint64(existingChassis.AssetSymbol12))) // 3rd person is now Sym12
			args = append(args, "--rig-hash", fmt.Sprintf("%d", uint64(existingChassis.AssetSymbol6)))
			args = append(args, "--mat-hash", fmt.Sprintf("%d", uint64(existingChassis.AssetSymbol11)))
			args = append(args, "--tex-hash", fmt.Sprintf("%d", uint64(existingChassis.AssetSymbol5))) // Texture is now Sym5
			
			hash3PHex := fmt.Sprintf("%016x", uint64(existingChassis.AssetSymbol12))
			baseMeshPath3P, err := state.FindBaseMesh(hash3PHex)
			if err == nil {
				args = append(args, "--base-mesh-3p", baseMeshPath3P)
			}
			
			hash1PHex := fmt.Sprintf("%016x", uint64(existingChassis.AssetSymbol6))
			baseMeshPath1P, err := state.FindBaseMesh(hash1PHex)
			if err == nil {
				args = append(args, "--base-mesh-1p", baseMeshPath1P)
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

		// Inject into CosmeticList if it's a new chassis
		fyne.Do(func() {
			if existingChassis == nil {
				newEntry := data.CosmeticEntry{}
				newEntry.CEntry = data.NewCDescriptor()
				
				// Base chassis settings
				newEntry.CEntry.CosmeticTypeSymbol = int64(data.ToSymbol("chassis"))
				intName := fmt.Sprintf("rwd_chassis_custom_%s", mf.MeshHashHex[:8])
				newEntry.CEntry.InternalNameSymbol = int64(data.ToSymbol(intName))
				newEntry.CEntry.InternalNameSymbol2 = newEntry.CEntry.InternalNameSymbol
				copy(newEntry.CEntry.InternalNameString[:], []byte(intName))
				copy(newEntry.CEntry.DisplayNameString[:], []byte("Custom Chassis"))
				newEntry.CEntry.RaritySymbol = data.HexToSymbol(data.RarityLegendary)
				
				// Inject asset symbols
				newEntry.CEntry.AssetSymbol5 = int64(mf.AssetSymbol12) // Python script maps texture to AssetSymbol12
				newEntry.CEntry.AssetSymbol6 = int64(mf.AssetSymbol6)
				newEntry.CEntry.AssetSymbol8 = -1
				newEntry.CEntry.AssetSymbol9 = -1
				newEntry.CEntry.AssetSymbol11 = int64(mf.AssetSymbol11)
				newEntry.CEntry.AssetSymbol12 = int64(mf.AssetSymbol5) // Python script maps 3rd person to AssetSymbol5
				newEntry.CEntry.AssetSymbol14 = -1
				newEntry.CEntry.AssetSymbol15 = -1
	
				// Append to list
				state.CosmeticList.CosmeticEntries = append(state.CosmeticList.CosmeticEntries, newEntry)
				state.CosmeticList.ListCount = uint64(len(state.CosmeticList.CosmeticEntries))
				state.CosmeticList.ListCount2 = state.CosmeticList.ListCount
				
				state.RefreshIndices()
				RefreshFilter(state, searchEntry.Text)
			} else {
				entry := &state.CosmeticList.CosmeticEntries[state.SelectedIndex]
				entry.CEntry.AssetSymbol5 = int64(mf.AssetSymbol12) // Texture
				entry.CEntry.AssetSymbol11 = int64(mf.AssetSymbol11)
				if variant == "1p" {
					entry.CEntry.AssetSymbol6 = int64(mf.AssetSymbol6) // 1st Person
				} else if variant == "3p" {
					entry.CEntry.AssetSymbol12 = int64(mf.AssetSymbol5) // 3rd Person
				} else {
					entry.CEntry.AssetSymbol6 = int64(mf.AssetSymbol6) // 1st Person
					entry.CEntry.AssetSymbol12 = int64(mf.AssetSymbol5) // 3rd Person
				}
				LoadToEditor(state, state.SelectedIndex)
			}
			
			// Attempt to copy the files to the Settings/input-pcvr directory using Rad Hex folders
			destGeoDirGPU := filepath.Join(data.GetSettingsDir(), "input-pcvr", "e642bfb1abcf76df") // CGMeshListResource GPU
			destGeoDirPri := filepath.Join(data.GetSettingsDir(), "input-pcvr", "37102e4b27955a14") // Chassis Primary
			destTexDir := filepath.Join(data.GetSettingsDir(), "input-pcvr", "4a4c32c49300b8a0")    // CTextureResource GPU
			
			os.MkdirAll(destGeoDirGPU, 0755)
			os.MkdirAll(destGeoDirPri, 0755)
			os.MkdirAll(destTexDir, 0755)
			
			// The python script now exports the GPU, Primary, Rig, and ALL texture DDS files directly 
			// to the input-pcvr directories above using the TextureReplacer from the addon!

			msg := "Custom Chassis added!\nThe mesh and textures were copied to Settings/input-pcvr.\nSave your dat file and repack the game."
			if existingChassis != nil {
				msg = "Chassis Model replaced!\nThe new mesh and textures were copied to Settings/input-pcvr to override the existing item.\nRepack your game."
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
