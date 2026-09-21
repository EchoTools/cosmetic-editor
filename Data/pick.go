package data

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
)

// File and folder choosers.
//
// These used to drive a Windows.Forms dialog through PowerShell, which is
// Windows-only and blocked the caller until the user answered. Fyne's own
// dialogs work on every platform the app builds for, and they are asynchronous,
// so these take a callback rather than returning a path.
//
// On Android a chosen file arrives as a content:// URI with no filesystem path
// behind it, so the file is copied into the app's own storage and the callback
// is handed that copy. Callers can therefore keep treating the result as an
// ordinary path.

// pickedDir is where files chosen by the user are copied to.
func pickedDir() string {
	dir := filepath.Join(GetSettingsDir(), "Temp", "Picked")
	os.MkdirAll(dir, 0755)
	return dir
}

// PickFile asks the user for a file and calls onPick with a readable path.
// onPick is not called if the user cancels or the copy fails.
func PickFile(state *AppState, extensions []string, onPick func(path string)) {
	if state == nil || state.Window == nil {
		return
	}
	d := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, state.Window)
			return
		}
		if rc == nil {
			return // cancelled
		}
		defer rc.Close()

		path, err := copyToPicked(rc)
		if err != nil {
			dialog.ShowError(err, state.Window)
			return
		}
		onPick(path)
	}, state.Window)

	if len(extensions) > 0 {
		d.SetFilter(storage.NewExtensionFileFilter(extensions))
	}
	d.Show()
}

// copyToPicked copies a chosen file into the app's storage and returns its path.
func copyToPicked(rc fyne.URIReadCloser) (string, error) {
	name := filepath.Base(rc.URI().Name())
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "picked"
	}
	// The URI name is user-controlled, so keep only the leaf and strip anything
	// that could climb out of the destination folder.
	name = strings.NewReplacer("/", "_", "\\", "_", "..", "_").Replace(name)

	dst := filepath.Join(pickedDir(), name)
	f, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, rc); err != nil {
		return "", err
	}
	return dst, nil
}

// PickFolder asks the user for a folder and calls onPick with its path.
//
// Android has no general folder picker that yields a filesystem path, so on a
// headset this is not the way the game directory is found: it is detected. The
// dialog is still offered for the desktop build and for the case where the
// detection misses.
func PickFolder(state *AppState, title string, onPick func(path string)) {
	if state == nil || state.Window == nil {
		return
	}
	d := dialog.NewFolderOpen(func(list fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, state.Window)
			return
		}
		if list == nil {
			return // cancelled
		}
		if p := list.Path(); p != "" {
			onPick(p)
		}
	}, state.Window)
	if title != "" {
		d.SetConfirmText(title)
	}
	d.Show()
}
