package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// findList returns the first list in a widget tree.
func findList(o fyne.CanvasObject) *widget.List {
	switch v := o.(type) {
	case *widget.List:
		return v
	case *fyne.Container:
		for _, c := range v.Objects {
			if l := findList(c); l != nil {
				return l
			}
		}
	case fyne.Widget:
		for _, c := range test.WidgetRenderer(v).Objects() {
			if l := findList(c); l != nil {
				return l
			}
		}
	}
	return nil
}

// findButtons collects every button in a widget tree, keyed by its label.
func findButtons(o fyne.CanvasObject, out map[string]*widget.Button) {
	switch v := o.(type) {
	case *widget.Button:
		out[v.Text] = v
	case *fyne.Container:
		for _, c := range v.Objects {
			findButtons(c, out)
		}
	case fyne.Widget:
		r := test.WidgetRenderer(v)
		for _, c := range r.Objects() {
			findButtons(c, out)
		}
	}
}

// startUI builds the real UI in the given mode with a throwaway settings file.
func startUI(t *testing.T, mode string) fyne.Window {
	t.Helper()
	a := test.NewApp()
	t.Cleanup(func() { a.Quit() })
	dir := t.TempDir()
	settingsFile = filepath.Join(dir, "settings.json")
	tempFilePath = filepath.Join(dir, "autosave.dat")
	if err := os.WriteFile(settingsFile, []byte(`{"mode":"`+mode+`"}`), 0644); err != nil {
		t.Fatal(err)
	}
	w := a.NewWindow("test")
	buildUI(a, w, dir)
	w.Show()

	// The cosmetic database loads in the background.
	deadline := time.Now().Add(5 * time.Second)
	for state.CosmeticList.CosmeticEntries == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if state.CosmeticList.CosmeticEntries == nil {
		t.Fatal("cosmetic database never loaded")
	}
	return w
}

// TestGenerateThumbnailOnlyOnTints: the Generate Thumbnail button belongs to
// the Tints editor, and must not show on any other tab.
func TestGenerateThumbnailOnlyOnTints(t *testing.T) {
	for _, mode := range []string{"Quest", "PCVR"} {
		t.Run(mode, func(t *testing.T) {
			w := startUI(t, mode)
			buttons := map[string]*widget.Button{}
			findButtons(w.Content(), buttons)

			for _, tab := range []string{"Tints", "Titles", "Emissives", "Emotes", "Banners", "Tags",
				"Emblems", "Decals", "Medals", "Pips", "Patterns", "Chassis", "Bracers", "Boosters", "Fanfares"} {
				b, ok := buttons[tab]
				if !ok {
					continue // not offered in this mode
				}
				test.Tap(b)
				check := func(when string) {
					shown := state.GenThumbBtn.Visible() && state.ThumbnailCard.Visible()
					if want := tab == "Tints"; shown != want {
						t.Errorf("%s tab, %s: Generate Thumbnail shown=%v, want %v", tab, when, shown, want)
					}
				}
				check("after switching tab")
				// Then pick an item, as a user does.
				if l := findList(w.Content()); l != nil && l.Length() > 0 {
					l.Select(0)
					check("after selecting an item")
				}
			}
		})
	}
}
