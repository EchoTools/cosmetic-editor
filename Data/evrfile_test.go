package data

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExtractKindTypeHashes checks that the extract filters are built from the
// platform layer rather than written out, so they cannot drift from the folder
// names the rest of the app uses.
func TestExtractKindTypeHashes(t *testing.T) {
	tints := ExtractTints.typeHashes()
	if len(tints) != 2 {
		t.Fatalf("tints filter has %d types, want both platforms", len(tints))
	}
	if tints[0] != HexToSymbol(PlatformPC.CosmeticDBTypeHash()) ||
		tints[1] != HexToSymbol(PlatformQuest.CosmeticDBTypeHash()) {
		t.Error("tints filter does not match the platform cosmetic DB type hashes")
	}

	textures := ExtractTextures.typeHashes()
	if len(textures) != 4 {
		t.Fatalf("textures filter has %d types, want header and GPU for both platforms", len(textures))
	}
	for _, want := range []string{
		PlatformPC.TextureMetaTypeHash(), PlatformPC.TextureGPUTypeHash(),
		PlatformQuest.TextureMetaTypeHash(), PlatformQuest.TextureGPUTypeHash(),
	} {
		found := false
		for _, got := range textures {
			if got == HexToSymbol(want) {
				found = true
			}
		}
		if !found {
			t.Errorf("textures filter is missing %s", want)
		}
	}
}

// TestExtractPackageFromRealInstall runs the in-process extractor against a
// real install. Opt in with EVR_DATA_DIR; the install is only read.
func TestExtractPackageFromRealInstall(t *testing.T) {
	dataDir := os.Getenv("EVR_DATA_DIR")
	if dataDir == "" {
		t.Skip("set EVR_DATA_DIR to a game data directory to run this")
	}
	if _, err := os.Stat(ManifestPath(dataDir)); err != nil {
		t.Skipf("no manifest at %s: %v", ManifestPath(dataDir), err)
	}

	out := t.TempDir()
	if err := ExtractPackage(dataDir, out, ExtractTints); err != nil {
		t.Fatalf("ExtractPackage: %v", err)
	}

	// The cosmetic database must come out under its own type folder.
	var found []string
	err := filepath.Walk(out, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			found = append(found, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 {
		t.Fatal("extract produced no files")
	}
	t.Logf("extracted %d files, e.g. %s", len(found), found[0])

	// The layout has to be <output>/<type hash>/<asset hash>, because that is
	// what FindExtractedAsset looks for; a grouping folder above the type makes
	// every extracted asset invisible to the rest of the app.
	for _, p := range found {
		rel, err := filepath.Rel(out, p)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(filepath.SplitList(rel)); got != 1 {
			t.Fatalf("unexpected relative path %q", rel)
		}
		dir, _ := filepath.Split(rel)
		if depth := len(splitPath(filepath.Clean(dir))); depth != 1 {
			t.Errorf("asset at %q sits %d folders below the root, want 1 (the type hash)", rel, depth)
		}
	}

	// And the cosmetic database must be findable the way the app looks for it.
	if p := FindExtractedAsset(out, HexToSymbol(PlatformPC.CosmeticDBAssetHash()),
		PlatformPC.CosmeticDBTypeHash()); p == "" {
		t.Error("FindExtractedAsset could not locate the extracted cosmetic database")
	}
}

// splitPath breaks a cleaned relative path into its components.
func splitPath(p string) []string {
	var out []string
	for {
		dir, file := filepath.Split(p)
		if file != "" {
			out = append([]string{file}, out...)
		}
		if dir == "" {
			return out
		}
		p = filepath.Clean(dir)
		if p == "." || p == string(filepath.Separator) {
			return out
		}
	}
}
