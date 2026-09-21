package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	data "github.com/EchoTools/cosmetic-editor/Data"
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
	t.Cleanup(func() {
		// Let background texture decodes finish first: one landing after the
		// app is gone would call into a dead driver.
		deadline := time.Now().Add(10 * time.Second)
		for data.TexturesInFlight() > 0 && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		a.Quit()
	})
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

// findEntries collects the text entries in a widget tree, in order.
func findEntries(o fyne.CanvasObject, out *[]*widget.Entry) {
	switch v := o.(type) {
	case *widget.Entry:
		*out = append(*out, v)
	case *fyne.Container:
		for _, c := range v.Objects {
			findEntries(c, out)
		}
	case fyne.Widget:
		for _, c := range test.WidgetRenderer(v).Objects() {
			findEntries(c, out)
		}
	}
}

// TestTintEditSurvivesReselect types a new colour into a tint, moves to
// another tint and back, and checks the edit is still there, both in the
// editor and in the database saved to disk.
func TestTintEditSurvivesReselect(t *testing.T) {
	for _, mode := range []string{"Quest", "PCVR"} {
		t.Run(mode, func(t *testing.T) {
			w := startUI(t, mode)
			buttons := map[string]*widget.Button{}
			findButtons(w.Content(), buttons)
			test.Tap(buttons["Tints"])
			l := findList(w.Content())
			if l == nil || l.Length() < 2 {
				t.Fatal("no tint list")
			}
			l.Select(0)
			idx := state.SelectedIndex

			var entries []*widget.Entry
			findEntries(state.CategoryEditor, &entries)
			if len(entries) < 2 {
				t.Fatalf("tint editor has %d entries, want 2", len(entries))
			}
			primary := entries[0]

			// What happened on the headset: letters that are not hex digits.
			// They must not end up in the field.
			primary.SetText("")
			test.Type(primary, "E4tgh")
			if primary.Text != "E4" {
				t.Errorf("non-hex input left %q in the field, want E4", primary.Text)
			}

			primary.SetText("")
			test.Type(primary, "12ab34")
			if primary.Text != "12AB34" {
				t.Fatalf("typed text = %q, want 12AB34", primary.Text)
			}

			l.Select(1)
			l.Select(0)
			entries = nil
			findEntries(state.CategoryEditor, &entries)
			if got := entries[0].Text; got != "12AB34" {
				t.Errorf("after moving away and back the colour is %q, want 12AB34", got)
			}

			// And it was saved to disk, so it survives the app being closed.
			b, err := os.ReadFile(state.CosmeticDBPaths()[0])
			if err != nil {
				t.Fatalf("no saved database: %v", err)
			}
			saved, err := data.BytesToCosmeticList(b)
			if err != nil {
				t.Fatal(err)
			}
			_, second, ok := data.TintThumbnailColors(saved.CosmeticEntries[idx])
			if !ok || second.R != 0x12 || second.G != 0xAB || second.B != 0x34 {
				t.Errorf("saved database holds %v for the edited colour, want 12AB34", second)
			}
		})
	}
}
