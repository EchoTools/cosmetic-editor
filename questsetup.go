package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	data "github.com/EchoTools/cosmetic-editor/Data"
)

// appID is the Android package name, which the system settings screens need.
const appID = "com.echotools.cosmeticeditor"

// setupPromptOpen stops the access and data prompts stacking up when the app
// is brought back to the foreground while one is already showing.
var setupPromptOpen bool

// dataReady is set once the game data has been found with access granted.
var dataReady bool

// checkQuestSetup makes sure the app can reach Echo VR's data on the headset.
//
// The data belongs to the game, so the app needs "All files access", which
// only the user can switch on; without it the game's files cannot even be
// seen. This runs at startup and every time the app comes back to the
// foreground, so returning from the settings screen picks the change up. Once
// access is there, the data directory is detected again.
func checkQuestSetup() {
	go func() {
		granted := data.HasAllFilesAccess()
		var all []string
		if granted {
			all = data.FindQuestDataPaths()
		}
		found := ""
		if len(all) > 0 {
			found = all[0]
		}
		fyne.Do(func() {
			if !granted {
				promptForFileAccess()
				return
			}
			if found == "" {
				promptDataNotFound()
				return
			}
			// Only act when something actually changed. This runs every time
			// the app regains focus, and on a headset that includes the system
			// keyboard opening and closing; rebuilding the open editor then
			// threw away whatever was half-typed in it.
			if dataReady && state.Settings.EchoVRDataPath == found {
				return
			}
			dataReady = true
			if state.Settings.EchoVRDataPath != found {
				state.Settings.EchoVRDataPath = found
				saveSettings()
			}
			// Anything read before access was granted failed; start clean, and
			// parse the package index now rather than on the first preview.
			data.ClosePackageReader()
			data.WarmPackageReader(found)
			if len(all) > 1 {
				state.StatusLabel.SetText(fmt.Sprintf("Echo VR data found in %d places; repacks mod all of them.", len(all)))
			} else {
				state.StatusLabel.SetText("Echo VR data found.")
			}
			if state.RefreshCurrent != nil {
				state.RefreshCurrent(state)
			}
		})
	}()
}

func promptForFileAccess() {
	if setupPromptOpen {
		return
	}
	setupPromptOpen = true
	w := state.Window
	d := dialog.NewConfirm("File access needed",
		"EchoVR Cosmetics needs \"All files access\" to read Echo VR's files "+
			"and repack your changes into them.\n\n"+
			"Tap Allow, switch on access for EchoVR Cosmetics, then come back to this app.",
		func(ok bool) {
			setupPromptOpen = false
			if !ok {
				return
			}
			go func() {
				if err := data.RequestAllFilesAccess(appID); err != nil {
					fyne.Do(func() {
						dialog.ShowError(fmt.Errorf("%v\n\nYou can also grant it from a PC with:\n"+
							"adb shell appops set %s MANAGE_EXTERNAL_STORAGE allow", err, appID), w)
					})
				}
			}()
		}, w)
	d.SetConfirmText("Allow")
	d.SetDismissText("Later")
	d.Show()
}

func promptDataNotFound() {
	if setupPromptOpen {
		return
	}
	setupPromptOpen = true
	dialog.ShowCustomConfirm("Echo VR data not found", "Check again", "Close",
		widget.NewLabel("File access is on, but Echo VR's data was not found on this headset.\n\n"+
			"Make sure Echo VR is installed and has been started at least once."),
		func(again bool) {
			setupPromptOpen = false
			if again {
				checkQuestSetup()
			}
		}, state.Window)
}
