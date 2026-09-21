package data

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// CopyRecursive copies a directory or file from src to dst.
func CopyRecursive(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
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

// ExecuteRepackTool handles the building and repacking of modified assets.
func ExecuteRepackTool(state *AppState, echoDataPath string) (string, error) {
	settingsPath := GetSettingsDir()
	absInputDir := state.StagingDir()
	absOutputDir := filepath.Join(settingsPath, OutputDirName)

	// Stage the cosmetic database under every asset name this platform
	// publishes it as.
	if err := state.SaveCosmeticDB(); err != nil {
		return "", fmt.Errorf("failed to stage cosmetic database: %v", err)
	}

	// 1. Specialized Backup logic: Backup original manifest BEFORE tool runs
	manifestPath := filepath.Join(echoDataPath, "manifests", PackageName)
	bakPath := manifestPath + ".bak"
	if _, err := os.Stat(bakPath); os.IsNotExist(err) {
		// Only create if it doesn't exist to preserve original state
		if data, err := os.ReadFile(manifestPath); err == nil {
			os.WriteFile(bakPath, data, 0644)
			// Record how many chunks the install had before we appended any,
			// so a later revert removes exactly the ones we added.  This has to
			// happen here, while the install is still untouched.
			stock := CountPackageChunks(echoDataPath)
			if err := RecordStockChunkCount(bakPath, stock); err != nil {
				fmt.Printf("[Backup] Could not record chunk count: %v\n", err)
			}
			fmt.Printf("[Backup] Created manifest backup: %s (%d stock chunks)\n", bakPath, stock)
		}
	}

	// Merge the staged files back into the install, in process.  The package
	// reader has to let go of the chunks first: the repack deletes stale ones.
	ClosePackageReader()
	if err := os.MkdirAll(absOutputDir, 0755); err != nil {
		return "", err
	}
	if err := RepackInto(echoDataPath, absInputDir); err != nil {
		return "", err
	}
	state.NeedsRepack = false // Reset repack tracking
	return fmt.Sprintf("Repacked %s into %s", filepath.Base(absInputDir), echoDataPath), nil
}

// ShowRepackDialog shows the repack and revert controls.
func ShowRepackDialog(state *AppState) {
	w := state.Window
	content := container.NewVBox()
	modal := dialog.NewCustom("Repack Tool", "Close", content, w)
	modal.Resize(fyne.NewSize(600, 450))

	var refreshUI func()
	refreshUI = func() {
		content.Objects = nil
		content.Add(widget.NewLabelWithStyle("Repack changes into the game", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))

		dataDir := state.Settings.EchoVRDataPath
		if !HasManifest(dataDir) {
			content.Add(widget.NewLabel("Echo VR's data directory was not found:\n" + dataDir))
			content.Refresh()
			return
		}

		manifestPath := ManifestPath(dataDir)
		bakPath := manifestPath + ".bak"
		_, errBak := os.Stat(bakPath)

		revertUI := container.NewVBox()
		if errBak == nil {
			revertUI.Add(widget.NewLabel("Backup manifest found."))
			btnRevert := widget.NewButton("Revert to Backup", func() {
				loading := dialog.NewCustom("Reverting...", "Cancel", widget.NewProgressBarInfinite(), w)
				loading.Show()
				go func() {
					defer loading.Hide()

					// 1. Put the original manifest back.  Close the package
					// reader first, or its open chunks cannot be deleted.
					ClosePackageReader()
					os.Remove(manifestPath)
					if err := os.Rename(bakPath, manifestPath); err != nil {
						fyne.Do(func() { dialog.ShowError(err, w) })
						return
					}
					// 2. Delete the chunks the repack appended.  A PC install
					// ships _0.._2 so the first repack appends _3, while a Quest
					// install ships only _0 and appends _1; deleting a fixed _3
					// removes the wrong file on Quest and misses later chunks on a
					// twice-repacked PC.
					stock := StockChunkCount(bakPath, state.Platform())
					for n := stock; ; n++ {
						chunk := PackageChunkPath(dataDir, n)
						if _, err := os.Stat(chunk); err != nil {
							break
						}
						os.Remove(chunk)
					}
					os.Remove(bakPath + chunkCountSuffix)

					fyne.Do(func() {
						dialog.ShowInformation("Success", "Manifest restored and modified package chunks removed.", w)
						refreshUI()
					})
				}()
			})
			btnRevert.Importance = widget.WarningImportance
			revertUI.Add(btnRevert)
		} else {
			revertUI.Add(widget.NewLabel("No manifest backup found yet.\n(Backup is created automatically during repack)"))
		}
		content.Add(widget.NewCard("Revert Management", "", revertUI))
		content.Add(widget.NewSeparator())

		content.Add(widget.NewLabel("Ready to repack changes into the game."))
		content.Add(widget.NewButton("REPACK & APPLY", func() {
			loading := dialog.NewCustom("Repacking...", "Cancel", widget.NewProgressBarInfinite(), w)
			loading.Show()
			go func() {
				output, err := ExecuteRepackTool(state, dataDir)
				fyne.Do(func() {
					loading.Hide()
					if err != nil {
						dialog.ShowError(err, w)
						return
					}
					dialog.ShowInformation("Success", "Assets repacked and applied to game!", w)
					fmt.Println(output)
					refreshUI()
				})
			}()
		}))
		content.Refresh()
	}

	refreshUI()
	modal.Show()
}
