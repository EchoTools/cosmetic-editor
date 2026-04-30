package data

import (
	"fmt"
	"io"
	"os"
	"os/exec"
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

// RunExtract handles the extraction of assets from the game files.
func RunExtract(state *AppState, echoDataPath string, exports string) error {
	settingsPath := GetSettingsDir()
	toolPath, err := FindTool(settingsPath, "evrFileTools.exe")
	if err != nil {
		return err
	}

	extractDir := filepath.Join(settingsPath, ExtractedDirName)
	os.MkdirAll(extractDir, 0755)

	cmd := exec.Command(toolPath,
		"-mode", "extract",
		"-package", PackageName,
		"-data", echoDataPath,
		"-output", extractDir,
		"-export", exports,
		"-force",
	)
	cmd.SysProcAttr = HiddenProcAttr()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("extraction failed: %v\nOutput: %s", err, string(out))
	}
	return nil
}

// ExecuteRepackTool handles the building and repacking of modified assets.
func ExecuteRepackTool(state *AppState, echoDataPath string) (string, error) {
	settingsPath := GetSettingsDir()
	inputDir := InputDirNamePC
	outputDir := OutputDirName
	tintFolder := TintFolderPC
	tintFile := TintFileNamePC
	if state.Settings.Mode == "Quest" {
		inputDir = InputDirNameQuest
		tintFolder = TintFolderQuest
		tintFile = TintFileNameQuest
	}

	absInputDir := filepath.Join(settingsPath, inputDir)
	absOutputDir := filepath.Join(settingsPath, outputDir)

	// Prepare Input File
	tintDir := filepath.Join(absInputDir, tintFolder)
	os.MkdirAll(tintDir, 0755)
	outFile := filepath.Join(tintDir, tintFile)

	b, err := CosmeticListToBytes(state.CosmeticList)
	if err != nil {
		return "", fmt.Errorf("failed to serialize tint data: %v", err)
	}
	if err := os.WriteFile(outFile, b, 0644); err != nil {
		return "", fmt.Errorf("failed to write data file: %v", err)
	}

	// 1. Specialized Backup logic: Backup original manifest BEFORE tool runs
	manifestPath := filepath.Join(echoDataPath, "manifests", PackageName)
	bakPath := manifestPath + ".bak"
	if _, err := os.Stat(bakPath); os.IsNotExist(err) {
		// Only create if it doesn't exist to preserve original state
		if data, err := os.ReadFile(manifestPath); err == nil {
			os.WriteFile(bakPath, data, 0644)
			fmt.Printf("[Backup] Created manifest backup: %s\n", bakPath)
		}
	}

	// Run evrFileTools
	os.MkdirAll(absOutputDir, 0755)
	toolPath, err := FindTool(settingsPath, "evrFileTools.exe")
	if err != nil {
		return "", err
	}

	cmd := exec.Command(toolPath,
		"-mode", "build",
		"-package", PackageName,
		"-data", echoDataPath,
		"-input", absInputDir,
		"-output", absOutputDir,
		"-force",
		"-quick",
	)
	cmd.SysProcAttr = HiddenProcAttr()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("repack failed: %v\nOutput: %s", err, string(out))
	}
	return string(out), nil
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
					err := RunExtract(state, state.Settings.EchoVRDataPath, "tints")
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
						// 3. Delete _3 package chunk
						chunk3 := filepath.Join(state.Settings.EchoVRDataPath, "packages", PackageName+"_3")
						os.Remove(chunk3)

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
