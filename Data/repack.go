package data

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

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
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
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
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
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

	refreshUI := func() {
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
			
			// Backup Section
			backupDir := filepath.Join(settingsPath, BackupDirName)
			_, errBackup := os.Stat(backupDir)
			backupExists := errBackup == nil

			backupUI := container.NewVBox()
			if backupExists {
				backupUI.Add(widget.NewLabel("Backup found."))
				btnRevert := widget.NewButton("Revert to Backup", func() {
					loading := dialog.NewCustom("Restoring Backup...", "Cancel", widget.NewProgressBarInfinite(), w)
					loading.Show()
					go func() {
						err := CopyRecursive(backupDir, state.Settings.EchoVRDataPath)
						loading.Hide()
						if err != nil {
							dialog.ShowError(err, w)
						} else {
							dialog.ShowInformation("Success", "Game files reverted to backup.", w)
						}
					}()
				})
				btnRevert.Importance = widget.WarningImportance
				backupUI.Add(btnRevert)
			} else {
				backupUI.Add(widget.NewLabel("No backup found."))
				backupUI.Add(widget.NewButton("Create Backup", func() {
					loading := dialog.NewCustom("Backing up...", "Cancel", widget.NewProgressBarInfinite(), w)
					loading.Show()
					go func() {
						os.MkdirAll(backupDir, 0755)
						err := CopyRecursive(state.Settings.EchoVRDataPath, backupDir)
						loading.Hide()
						if err != nil {
							dialog.ShowError(err, w)
						} else {
							dialog.ShowInformation("Backup", "Backup created successfully.", w)
							ShowRepackDialog(state)
						}
					}()
				}))
			}
			content.Add(widget.NewCard("Backup Management", "", backupUI))
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
