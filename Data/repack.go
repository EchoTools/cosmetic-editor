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

// RunExtract pulls the cosmetic database and textures out of the install.
func RunExtract(state *AppState, echoDataPath string) error {
	extractDir := filepath.Join(GetSettingsDir(), state.Platform().ExtractedDirName())
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return err
	}
	return ExtractPackage(echoDataPath, extractDir, ExtractTints, ExtractTextures)
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

	// Merge the staged files back into the install, in process.
	if err := os.MkdirAll(absOutputDir, 0755); err != nil {
		return "", err
	}
	if err := RepackInto(echoDataPath, absInputDir); err != nil {
		return "", err
	}
	state.NeedsRepack = false // Reset repack tracking
	return fmt.Sprintf("Repacked %s into %s", filepath.Base(absInputDir), echoDataPath), nil
}

// ShowRepackDialog displays the multi-step repack UI to the user.
func ShowRepackDialog(state *AppState) {
	w := state.Window
	content := container.NewVBox()
	modal := dialog.NewCustom("Repack Tool", "Close", content, w)
	modal.Resize(fyne.NewSize(600, 450))

	settingsPath := GetSettingsDir()
	extractDir := filepath.Join(settingsPath, ExtractedDirName)
	_, errExtract := os.Stat(extractDir)
	extractedExists := errExtract == nil

	var refreshUI func()
	refreshUI = func() {
		content.Objects = nil

		if !extractedExists {
			content.Add(widget.NewLabel("Step 1: Extract Original Tints"))
			content.Add(widget.NewLabel("Selected EchoVR Data Path:"))
			content.Add(widget.NewLabel(state.Settings.EchoVRDataPath))
			content.Add(widget.NewButton("Extract", func() {
				loading := dialog.NewCustom("Extracting...", "Cancel", widget.NewProgressBarInfinite(), w)
				loading.Show()
				go func() {
					err := RunExtract(state, state.Settings.EchoVRDataPath)
					loading.Hide()
					if err != nil {
						dialog.ShowError(err, w)
					} else {
						extractedExists = true
						ShowRepackDialog(state)
					}
				}()
			}))
		} else {
			content.Add(widget.NewLabelWithStyle("Step 2: Modify & Repack", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))

			// Revert Section
			manifestPath := filepath.Join(state.Settings.EchoVRDataPath, "manifests", PackageName)
			bakPath := manifestPath + ".bak"
			_, errBak := os.Stat(bakPath)
			bakExists := errBak == nil

			revertUI := container.NewVBox()
			if bakExists {
				revertUI.Add(widget.NewLabel("Backup manifest found."))
				btnRevert := widget.NewButton("Revert to Backup", func() {
					loading := dialog.NewCustom("Reverting...", "Cancel", widget.NewProgressBarInfinite(), w)
					loading.Show()
					go func() {
						defer loading.Hide()

						// 1. Delete current manifest
						os.Remove(manifestPath)
						// 2. Rename .bak to original
						err := os.Rename(bakPath, manifestPath)
						if err != nil {
							fyne.Do(func() { dialog.ShowError(err, w) })
							return
						}
						// 3. Delete the chunks the repack appended.  A PC
						// install ships _0.._2 so the first repack appends _3,
						// while a Quest install ships only _0 and appends _1;
						// deleting a fixed _3 removes the wrong file on Quest
						// and misses later chunks on a twice-repacked PC.
						stock := StockChunkCount(bakPath, state.Platform())
						for n := stock; ; n++ {
							chunk := PackageChunkPath(state.Settings.EchoVRDataPath, n)
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

			content.Add(widget.NewLabel("Ready to Repack changes into game."))
			content.Add(widget.NewButton("REPACK & APPLY", func() {
				loading := dialog.NewCustom("Repacking...", "Cancel", widget.NewProgressBarInfinite(), w)
				loading.Show()
				go func() {
					output, err := ExecuteRepackTool(state, state.Settings.EchoVRDataPath)
					loading.Hide()
					if err != nil {
						dialog.ShowError(err, w)
					} else {
						dialog.ShowInformation("Success", "Assets repacked and applied to game!", w)
						fmt.Println(output)
					}
				}()
			}))
		}
		content.Refresh()
	}

	refreshUI()
	modal.Show()
}
