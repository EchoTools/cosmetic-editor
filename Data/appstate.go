package data

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// AppState holds the global state of the application to facilitate modular UI.
type AppState struct {
	App      fyne.App
	Window   fyne.Window
	Settings AppSettings

	// Data
	CosmeticList CosmeticList
	
	// Indices and Filtered state for each category
	CategoryIndices  map[string][]int
	CategoryFiltered map[string][]int
	
	// Selection State
	SelectedCategory string
	SelectedIndex    int // index in the respective category list
	IsLoadingEntry   bool

	// Global UI Elements
	StatusLabel  *widget.Label
	NameEntry    *widget.Entry
	DescEntry    *widget.Entry
	ThumbIdEntry *widget.Entry
	ThumbIdItem  *widget.FormItem
	RaritySelect *widget.Select
	ThumbImage   *canvas.Image
	TextureImage *canvas.Image
	ReplaceBtn   *widget.Button
	GenThumbBtn  *widget.Button
	
	// Shared UI indicators or previews
	PreviewTintIndex int // For the preview tint feature
	PreviewTintCheck *widget.Check
	PreviewTintSelect *widget.Select
	PreviewTintContainer *fyne.Container

	// Animation Control
	CancelAnim context.CancelFunc

	// Asset State
	CurrentAssetSymbol string
	MainPreviewGroup   *fyne.Container
	ThumbnailCard      *fyne.Container
	MainPreviewCard    *fyne.Container

	// Category-specific dynamic editor (inserted into the sidebar)
	CategoryEditor *fyne.Container
	TintHexEntries [2]*widget.Entry

	// Emissive Preview Components
	EmissivePreviewLabel   *widget.Label
	EmissivePreviewWrapper *fyne.Container
	EmissivePreviewImage   *canvas.Image

	// Callback to refresh the currently active category-specific editor
	RefreshCurrent func(*AppState)

	// Persisted paths for previewing
	CurrentOriginalAssetPath string
	CurrentReplacementPath   string
}
// UpdateSidebarThumbnail refreshes the sidebar thumbnail from the local texture cache.
func (s *AppState) UpdateSidebarThumbnail(symbol int64) {
	if s.ThumbImage == nil {
		return
	}
	if symbol == 0 || symbol == -1 {
		s.ThumbImage.Resource = nil
		s.ThumbImage.Image = nil
		s.ThumbImage.Refresh()
		return
	}

	hexStr := SymbolToHex(symbol)
	EnsureTextureCached(s, hexStr)
	cachePath := filepath.Join(s.Settings.TextureCachePath, hexStr+".png")

	if _, err := os.Stat(cachePath); err == nil {
		res, _ := fyne.LoadResourceFromPath(cachePath)
		s.ThumbImage.Resource = res
		s.ThumbImage.Image = nil
	} else {
		s.ThumbImage.Resource = nil
		s.ThumbImage.Image = nil
	}
	s.ThumbImage.Refresh()
}

// UpdateMainTexture refreshes the larger texture preview from the local cache.
func (s *AppState) UpdateMainTexture(symbol int64) {
	if s.TextureImage == nil {
		return
	}
	if symbol == 0 || symbol == -1 {
		s.TextureImage.Resource = nil
		s.TextureImage.Image = nil
		s.TextureImage.Refresh()
		return
	}

	hexStr := SymbolToHex(symbol)
	EnsureTextureCached(s, hexStr)
	cachePath := filepath.Join(s.Settings.TextureCachePath, hexStr+".png")

	if _, err := os.Stat(cachePath); err == nil {
		res, _ := fyne.LoadResourceFromPath(cachePath)
		s.TextureImage.Resource = res
		s.TextureImage.Image = nil
	} else {
		s.TextureImage.Resource = nil
		s.TextureImage.Image = nil
	}
	s.TextureImage.Refresh()
}

// ClearUI resets the common sidebar fields and clears any category-specific editor content.
func (s *AppState) ClearUI() {
	// Stop any running animations
	if s.CancelAnim != nil {
		s.CancelAnim()
		s.CancelAnim = nil
	}

	s.SelectedIndex = -1
	s.IsLoadingEntry = true
	s.NameEntry.SetText("")
	s.DescEntry.SetText("")
	s.ThumbIdEntry.SetText("")
	s.RaritySelect.SetSelected("")
	s.UpdateSidebarThumbnail(0)
	s.UpdateMainTexture(0)
	
	if s.CategoryEditor != nil {
		s.CategoryEditor.Objects = nil
		s.CategoryEditor.Refresh()
	}
	
	if s.ThumbIdItem != nil {
		s.ThumbIdItem.Widget.Show()
	}

	if s.MainPreviewGroup != nil {
		s.MainPreviewGroup.Hide()
	}

	if s.ReplaceBtn != nil {
		s.ReplaceBtn.Hide()
	}

	s.CurrentAssetSymbol = ""
	s.CurrentOriginalAssetPath = ""
	s.CurrentReplacementPath = ""

	s.IsLoadingEntry = false
}

func NewAppState(a fyne.App, w fyne.Window) *AppState {
	return &AppState{
		App:              a,
		Window:           w,
		CategoryIndices:  make(map[string][]int),
		CategoryFiltered: make(map[string][]int),
		SelectedIndex:    -1,
		PreviewTintIndex: -1,
	}
}

// AppSettings moved from main.go
type AppSettings struct {
	EchoVRDataPath   string `json:"echovr_data_path"`
	ExtractedPath    string `json:"extracted_path"`
	Mode             string `json:"mode"`
	TextureCachePath string `json:"texture_cache_path"`

	// These were used in original for various functions
	TextureOutPath  string `json:"texture_out_path"`
	MetadataOutPath string `json:"metadata_out_path"`
	BackupPath      string `json:"backup_path"`
}

var RaritySymbolToName = map[int64]string{
	0x3cf7320f92500000: "Default",
	0x34cc320f92500000: "Common",
	0x34cd320f92500000: "Uncommon",
	0x34ce320f92500000: "Rare",
	0x34cf320f92500000: "Epic",
	0x34d0320f92500000: "Legendary",
	0x34d1320f92500000: "Exotic",
}

// RefreshIndices categorizes all cosmetic entries into their respective lists.
func (s *AppState) RefreshIndices() {
	s.CategoryIndices = make(map[string][]int)
	s.CategoryFiltered = make(map[string][]int)

	tintSym := int64(ToSymbol("tint"))
	titleSym := int64(ToSymbol("title"))
	emissiveSym := int64(ToSymbol("emissive"))
	emoteSym := int64(ToSymbol("emote"))
	bannerSym := int64(ToSymbol("banner"))
	tagSym := int64(ToSymbol("tag"))
	decalSym := int64(ToSymbol("decal"))
	medalSym := int64(ToSymbol("medal"))
	pipSym := int64(ToSymbol("pip"))
	patternSym := int64(ToSymbol("pattern"))

	for i, e := range s.CosmeticList.CosmeticEntries {
		// Identify fanfares first
		if e.CEntry.WWiseSoundBankID2 != 0 {
			s.CategoryIndices["Fanfares"] = append(s.CategoryIndices["Fanfares"], i)
			continue
		}

		switch e.CEntry.CosmeticTypeSymbol {
		case tintSym:
			// Emissive check (inherited logic from main.go)
			isEmissive := strings.Contains(strings.ToLower(string(bytes.TrimRight(e.CEntry.InternalNameString[:], "\x00"))), "emissive") ||
				e.CEntry.TextureSymbol != -1 ||
				len(e.CEntryExtData) != 24 ||
				e.CEntry.EmissiveUnk1 != 0 ||
				e.CEntry.EmissiveUnk2 != 0

			if !isEmissive {
				s.CategoryIndices["Tints"] = append(s.CategoryIndices["Tints"], i)
			} else {
				s.CategoryIndices["Emissives"] = append(s.CategoryIndices["Emissives"], i)
			}
		case emissiveSym:
			s.CategoryIndices["Emissives"] = append(s.CategoryIndices["Emissives"], i)
		case titleSym:
			s.CategoryIndices["Titles"] = append(s.CategoryIndices["Titles"], i)
		case emoteSym:
			s.CategoryIndices["Emotes"] = append(s.CategoryIndices["Emotes"], i)
		case bannerSym:
			s.CategoryIndices["Banners"] = append(s.CategoryIndices["Banners"], i)
		case tagSym:
			s.CategoryIndices["Tags"] = append(s.CategoryIndices["Tags"], i)
		case decalSym:
			s.CategoryIndices["Emblems"] = append(s.CategoryIndices["Emblems"], i)
		case medalSym:
			s.CategoryIndices["Medals"] = append(s.CategoryIndices["Medals"], i)
		case pipSym:
			s.CategoryIndices["Pips"] = append(s.CategoryIndices["Pips"], i)
		case patternSym:
			s.CategoryIndices["Patterns"] = append(s.CategoryIndices["Patterns"], i)
		}
	}

	// Initialize filtered indices with full indices
	for cat, idxs := range s.CategoryIndices {
		s.CategoryFiltered[cat] = append([]int{}, idxs...)
	}

	// Update Preview Tint Select options
	if s.PreviewTintSelect != nil {
		var tintNames []string
		for _, idx := range s.CategoryIndices["Tints"] {
			entry := s.CosmeticList.CosmeticEntries[idx]
			dName := strings.TrimRight(string(entry.CEntry.DisplayNameString[:]), "\x00")
			tintNames = append(tintNames, dName)
		}
		s.PreviewTintSelect.Options = tintNames
		s.PreviewTintSelect.Refresh()
	}
}

// HandleSave persists the current state of CosmeticList to the temp file.
func (s *AppState) HandleSave(tempPath string) error {
	data, err := CosmeticListToBytes(s.CosmeticList)
	if err != nil {
		return err
	}
	return os.WriteFile(tempPath, data, 0644)
}
