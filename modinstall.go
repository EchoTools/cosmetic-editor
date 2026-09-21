package main

import (
	"fmt"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	data "github.com/EchoTools/cosmetic-editor/Data"
)

// installOtherMod asks for a mod zip and adds its files to the staging folder,
// so the next repack applies them together with the editor's own changes.
func installOtherMod() {
	w := state.Window
	data.PickFile(state, []string{".zip"}, func(zipPath string) {
		loading := dialog.NewCustomWithoutButtons("Installing mod...", widget.NewProgressBarInfinite(), w)
		loading.Show()
		go func() {
			res, err := data.InstallModZip(zipPath, state.StagingDir(), state.Platform())
			// PickFile hands over a copy in the app's own storage; it is not
			// needed once the files are staged.
			os.Remove(zipPath)
			fyne.Do(func() {
				loading.Hide()
				if err != nil {
					dialog.ShowError(fmt.Errorf("could not install the mod: %w", err), w)
					return
				}
				// A replaced texture's old preview is cached; drop it so the
				// mod's version is shown.
				for _, t := range res.Textures {
					data.ForgetTexturePreview(state, t)
				}
				state.NeedsRepack = true
				if state.RefreshCurrent != nil {
					state.RefreshCurrent(state)
				}
				dialog.ShowInformation("Mod installed", modInstallSummary(res), w)
			})
		}()
	})
}

func modInstallSummary(res data.ModInstallResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Added %d file(s)", res.Installed)
	if res.Replaced > 0 {
		fmt.Fprintf(&b, ", replacing %d already waiting to be repacked", res.Replaced)
	}
	b.WriteString(".\n\nPress REPACK PACKAGE to apply the mod along with your own changes.")
	if res.SkippedDB {
		b.WriteString("\n\nThe mod also had its own cosmetic database, which was left out: " +
			"the editor writes the database on every repack, so make cosmetic changes here instead.")
	}
	return b.String()
}
