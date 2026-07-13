package bracers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	data "github.com/EchoTools/cosmetic-editor/Data"

	"io"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	bracerList  *widget.List
	searchEntry *widget.Entry
)

func SetupUI(state *data.AppState) fyne.CanvasObject {
	searchEntry = widget.NewEntry()
	searchEntry.PlaceHolder = "Search Bracers..."
	searchEntry.OnChanged = func(s string) {
		RefreshFilter(state, s)
	}

	bracerList = widget.NewList(
		func() int {
			return len(state.CategoryFiltered["Bracers"])
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("Template")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			realIdx := state.CategoryFiltered["Bracers"][id]
			entry := state.CosmeticList.CosmeticEntries[realIdx]
			dName := string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00"))
			item.(*widget.Label).SetText(dName)
		},
	)

	bracerList.OnSelected = func(id widget.ListItemID) {
		realIdx := state.CategoryFiltered["Bracers"][id]
		LoadToEditor(state, realIdx)
	}

	content := container.NewBorder(
		container.NewVBox(searchEntry),
		nil, nil, nil, bracerList,
	)

	return content
}

func LoadToEditor(state *data.AppState, realIdx int) {
	state.SelectedIndex = realIdx
	state.SelectedCategory = "Bracers"
	state.RefreshCurrent = func(s *data.AppState) { LoadToEditor(s, realIdx) }

	c := data.CBracer{}
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

	sym9Entry := widget.NewEntry()
	sym9Entry.SetText(fmt.Sprintf("%016x", uint64(c.AssetSymbol9)))
	sym6Entry := widget.NewEntry()
	sym6Entry.SetText(fmt.Sprintf("%016x", uint64(c.AssetSymbol6)))

	saveSymbols := func() {
		if state.IsLoadingEntry {
			return
		}
		entry := &state.CosmeticList.CosmeticEntries[state.SelectedIndex]

		entry.CEntry.AssetSymbol9 = data.HexToSymbol(sym9Entry.Text)
		entry.CEntry.AssetSymbol6 = data.HexToSymbol(sym6Entry.Text)
		state.AutoSave()
	}

	sym9Entry.OnChanged = func(string) { saveSymbols() }
	sym6Entry.OnChanged = func(string) { saveSymbols() }

	state.CategoryEditor.Objects = []fyne.CanvasObject{
		widget.NewForm(
			widget.NewFormItem("Left Model Hash (Sym9)", sym9Entry),
			widget.NewFormItem("Right Model Hash (Sym6)", sym6Entry),
		),
		widget.NewButtonWithIcon("Replace Model from .blend", theme.UploadIcon(), func() {
			c.AssetSymbol9 = data.HexToSymbol(sym9Entry.Text)
			c.AssetSymbol6 = data.HexToSymbol(sym6Entry.Text)
			importCustomBracer(state, &c)
		}),
	}
	state.CategoryEditor.Refresh()

	state.IsLoadingEntry = false
}

func RefreshFilter(state *data.AppState, query string) {
	query = strings.ToLower(query)
	state.CategoryFiltered["Bracers"] = []int{}
	for _, idx := range state.CategoryIndices["Bracers"] {
		entry := state.CosmeticList.CosmeticEntries[idx]
		dName := strings.ToLower(string(bytes.TrimRight(entry.CEntry.DisplayNameString[:], "\x00")))
		if query == "" || strings.Contains(dName, query) {
			state.CategoryFiltered["Bracers"] = append(state.CategoryFiltered["Bracers"], idx)
		}
	}
	if bracerList != nil {
		bracerList.Refresh()
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

func importCustomBracer(state *data.AppState, existingBracer *data.CBracer) {
	if existingBracer != nil {
		var dlg dialog.Dialog
		leftBtn := widget.NewButton("Left Bracer", func() {
			dlg.Hide()
			doImportBracer(state, existingBracer, "left")
		})
		rightBtn := widget.NewButton("Right Bracer", func() {
			dlg.Hide()
			doImportBracer(state, existingBracer, "right")
		})
		bothBtn := widget.NewButton("Both", func() {
			dlg.Hide()
			doImportBracer(state, existingBracer, "both")
		})
		dlg = dialog.NewCustom("Select Variant to Replace", "Cancel", container.NewVBox(leftBtn, rightBtn, bothBtn), state.Window)
		dlg.Show()
	} else {
		doImportBracer(state, existingBracer, "both")
	}
}

func doImportBracer(state *data.AppState, existingBracer *data.CBracer, variant string) {
	blendPath, err := data.PickFile("3D Models (*.blend;*.glb)|*.blend;*.glb|All Files (*.*)|*.*")
	if err != nil || blendPath == "" {
		return
	}

	outDir := filepath.Join(data.GetSettingsDir(), "Temp", "ChassisBake")
	os.MkdirAll(outDir, 0755)

	scriptPath := filepath.Join(data.GetSettingsDir(), "Temp", "Scripts", "backend_chassis_builder.py")

	loading := dialog.NewCustom("Processing Custom Bracer...", "Please wait, Blender is working...", widget.NewProgressBarInfinite(), state.Window)
	loading.Show()

	go func() {
		args := []string{scriptPath, blendPath, outDir, "--type", "bracer"}
		if existingBracer != nil {
			var meshSym int64
			if variant == "left" {
				meshSym = existingBracer.AssetSymbol9
			} else if variant == "right" {
				meshSym = existingBracer.AssetSymbol6
			} else {
				meshSym = existingBracer.AssetSymbol9 // Both bases on left
			}
			args = append(args, "--mesh-hash", fmt.Sprintf("%d", uint64(meshSym)))

			hashHex := fmt.Sprintf("%016x", uint64(meshSym))
			baseMeshPath, err := state.FindBaseMesh(hashHex)
			if err == nil {
				args = append(args, "--base-mesh-3p", baseMeshPath)
			}

			args = append(args, "--mat-hash", fmt.Sprintf("%d", uint64(existingBracer.AssetSymbol11)))
			args = append(args, "--tex-hash", fmt.Sprintf("%d", uint64(existingBracer.AssetSymbol5)))
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

		// Inject into CosmeticList if it's a new Bracer
		fyne.Do(func() {
			if existingBracer == nil {
				newEntry := data.CosmeticEntry{}
				newEntry.CEntry = data.NewCDescriptor()

				newEntry.CEntry.CosmeticTypeSymbol = int64(data.ToSymbol("bracer"))
				intName := fmt.Sprintf("rwd_bracer_custom_%s", mf.MeshHashHex[:8])
				newEntry.CEntry.InternalNameSymbol = int64(data.ToSymbol(intName))
				newEntry.CEntry.InternalNameSymbol2 = newEntry.CEntry.InternalNameSymbol
				copy(newEntry.CEntry.InternalNameString[:], []byte(intName))
				copy(newEntry.CEntry.DisplayNameString[:], []byte("Custom Bracer"))
				newEntry.CEntry.RaritySymbol = data.HexToSymbol(data.RarityLegendary)

				newEntry.CEntry.AssetSymbol5 = -1
				newEntry.CEntry.AssetSymbol6 = int64(mf.AssetSymbol5)
				newEntry.CEntry.AssetSymbol8 = -1
				newEntry.CEntry.AssetSymbol9 = int64(mf.AssetSymbol5)
				newEntry.CEntry.AssetSymbol11 = -1
				newEntry.CEntry.AssetSymbol12 = -1
				newEntry.CEntry.AssetSymbol14 = -1
				newEntry.CEntry.AssetSymbol15 = -1

				state.CosmeticList.CosmeticEntries = append(state.CosmeticList.CosmeticEntries, newEntry)
				state.CosmeticList.ListCount = uint64(len(state.CosmeticList.CosmeticEntries))
				state.CosmeticList.ListCount2 = state.CosmeticList.ListCount

				state.RefreshIndices()
				RefreshFilter(state, searchEntry.Text)
			} else {
				// Overwrite the existing entry based on variant
				entry := &state.CosmeticList.CosmeticEntries[state.SelectedIndex]
				if variant == "left" {
					entry.CEntry.AssetSymbol9 = int64(mf.AssetSymbol5)
				} else if variant == "right" {
					entry.CEntry.AssetSymbol6 = int64(mf.AssetSymbol5)
				} else {
					entry.CEntry.AssetSymbol9 = int64(mf.AssetSymbol5)
					entry.CEntry.AssetSymbol6 = int64(mf.AssetSymbol5)
				}
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

			msg := "Custom Bracer added!\nThe mesh and textures were copied to Settings/input-pcvr.\nSave your dat file and repack the game."
			if existingBracer != nil {
				msg = "Bracer Model replaced!\nThe new mesh and textures were copied to Settings/input-pcvr to override the existing item.\nRepack your game."
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
