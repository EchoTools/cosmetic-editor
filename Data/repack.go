package data

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

// DataPaths returns every game data directory a repack should be applied to.
// On a headset Echo VR can be installed under Android/media, under
// /sdcard/readyatdawn, or both, and every copy found is modded so the change
// shows up whichever one the game loads. The configured directory, which is
// the one previews are read from, always comes first.
func (state *AppState) DataPaths() []string {
	var out []string
	add := func(dir string) {
		if !HasManifest(dir) {
			return
		}
		info, err := os.Stat(ManifestPath(dir))
		if err != nil {
			return
		}
		for _, have := range out {
			if hi, err := os.Stat(ManifestPath(have)); err == nil && os.SameFile(hi, info) {
				return
			}
		}
		out = append(out, dir)
	}
	add(state.Settings.EchoVRDataPath)
	if IsAndroid() && state.Platform() == PlatformQuest {
		for _, dir := range FindQuestDataPaths() {
			add(dir)
		}
	}
	return out
}

// backupManifest saves an install's manifest, and how many package chunks it
// has, before the first repack touches it. An existing backup is left alone,
// since it is the one that describes the untouched install.
func backupManifest(echoDataPath string) {
	bakPath := ManifestPath(echoDataPath) + ".bak"
	if _, err := os.Stat(bakPath); !os.IsNotExist(err) {
		return
	}
	data, err := os.ReadFile(ManifestPath(echoDataPath))
	if err != nil {
		return
	}
	if err := os.WriteFile(bakPath, data, 0644); err != nil {
		fmt.Printf("[Backup] Could not back up %s: %v\n", echoDataPath, err)
		return
	}
	// Record how many chunks the install had before we appended any, so a
	// later revert removes exactly the ones we added.
	stock := CountPackageChunks(echoDataPath)
	if err := RecordStockChunkCount(bakPath, stock); err != nil {
		fmt.Printf("[Backup] Could not record chunk count: %v\n", err)
	}
	fmt.Printf("[Backup] Created manifest backup: %s (%d stock chunks)\n", bakPath, stock)
}

// ExecuteRepackTool stages the cosmetic database and repacks the staged files
// into each of the given game data directories. It stops at the first
// directory that fails and reports which ones were done.
func ExecuteRepackTool(state *AppState, echoDataPaths []string) (string, error) {
	if len(echoDataPaths) == 0 {
		return "", fmt.Errorf("no Echo VR data directory to repack into")
	}
	absInputDir := state.StagingDir()
	absOutputDir := filepath.Join(GetSettingsDir(), OutputDirName)

	// Stage the cosmetic database under every asset name this platform
	// publishes it as.
	if err := state.SaveCosmeticDB(); err != nil {
		return "", fmt.Errorf("failed to stage cosmetic database: %v", err)
	}
	if err := os.MkdirAll(absOutputDir, 0755); err != nil {
		return "", err
	}

	// The package reader has to let go of the chunks first: the repack
	// deletes stale ones.
	ClosePackageReader()
	var done []string
	for _, dir := range echoDataPaths {
		backupManifest(dir)
		if err := RepackInto(dir, absInputDir); err != nil {
			if len(done) > 0 {
				return "", fmt.Errorf("repacked %s, but %s failed: %v",
					strings.Join(done, ", "), dir, err)
			}
			return "", fmt.Errorf("%s: %v", dir, err)
		}
		done = append(done, dir)
	}
	state.NeedsRepack = false
	return "Repacked into:\n" + strings.Join(done, "\n"), nil
}

// revertInstall puts back an install's backed-up manifest and deletes the
// package chunks repacks appended to it. It does nothing to an install that
// has no backup.
func revertInstall(dataDir string, p Platform) error {
	manifestPath := ManifestPath(dataDir)
	bakPath := manifestPath + ".bak"
	if _, err := os.Stat(bakPath); err != nil {
		return nil
	}
	os.Remove(manifestPath)
	if err := os.Rename(bakPath, manifestPath); err != nil {
		return err
	}
	// A PC install ships _0.._2 so the first repack appends _3, while a Quest
	// install ships only _0 and appends _1. The count recorded at backup time
	// says where ours start.
	stock := StockChunkCount(bakPath, p)
	for n := stock; ; n++ {
		chunk := PackageChunkPath(dataDir, n)
		if _, err := os.Stat(chunk); err != nil {
			break
		}
		os.Remove(chunk)
	}
	os.Remove(bakPath + chunkCountSuffix)
	return nil
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

		dirs := state.DataPaths()
		if len(dirs) == 0 {
			content.Add(widget.NewLabel("Echo VR's data directory was not found:\n" + state.Settings.EchoVRDataPath))
			content.Refresh()
			return
		}

		where := "Echo VR install:"
		if len(dirs) > 1 {
			where = fmt.Sprintf("%d Echo VR installs found; all of them are modded:", len(dirs))
		}
		loc := widget.NewLabel(where + "\n" + strings.Join(dirs, "\n"))
		loc.Wrapping = fyne.TextWrapBreak
		content.Add(loc)

		var backedUp []string
		for _, dir := range dirs {
			if _, err := os.Stat(ManifestPath(dir) + ".bak"); err == nil {
				backedUp = append(backedUp, dir)
			}
		}

		revertUI := container.NewVBox()
		if len(backedUp) > 0 {
			revertUI.Add(widget.NewLabel(fmt.Sprintf("Backup manifest found (%d of %d installs).", len(backedUp), len(dirs))))
			btnRevert := widget.NewButton("Revert to Backup", func() {
				loading := dialog.NewCustom("Reverting...", "Cancel", widget.NewProgressBarInfinite(), w)
				loading.Show()
				go func() {
					defer fyne.Do(loading.Hide)
					// Close the package reader first, or its open chunks
					// cannot be deleted.
					ClosePackageReader()
					for _, dir := range backedUp {
						if err := revertInstall(dir, state.Platform()); err != nil {
							fyne.Do(func() { dialog.ShowError(fmt.Errorf("%s: %v", dir, err), w) })
							return
						}
					}
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
				output, err := ExecuteRepackTool(state, dirs)
				fyne.Do(func() {
					loading.Hide()
					if err != nil {
						dialog.ShowError(err, w)
						return
					}
					dialog.ShowInformation("Success", "Assets repacked and applied to game!\n\n"+output, w)
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
