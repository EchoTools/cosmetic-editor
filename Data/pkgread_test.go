package data

import (
	"os"
	"testing"
)

// TestReadTexturesStraightFromPackage decodes textures directly out of a real
// install's package, with nothing extracted. Opt in with EVR_DATA_DIR (and
// EVR_PLATFORM=Quest for an android install); the install is only read.
func TestReadTexturesStraightFromPackage(t *testing.T) {
	dataDir := os.Getenv("EVR_DATA_DIR")
	if dataDir == "" {
		t.Skip("set EVR_DATA_DIR to a game data directory to run this")
	}
	p := ParsePlatform(os.Getenv("EVR_PLATFORM"))
	if os.Getenv("EVR_PLATFORM") == "" {
		p = PlatformPC
	}

	r, err := openPackageReader(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	metaType := HexToSymbol(p.TextureMetaTypeHash())
	src := packageSource(dataDir)

	decoded, tried := 0, 0
	for key := range r.index {
		if key.typ != metaType {
			continue
		}
		tried++
		img, err := DecodeTextureAsset(p, src, SymbolToHex(key.file))
		if err == nil && img.Bounds().Dx() > 0 {
			decoded++
		} else if err != nil && tried <= 5 {
			t.Logf("  %s: %v", SymbolToHex(key.file), err)
		}
		if tried == 40 {
			break
		}
	}
	if tried == 0 {
		t.Fatalf("no %s texture headers in the package", p)
	}
	t.Logf("decoded %d of %d %s textures straight from the package", decoded, tried, p)
	if decoded == 0 {
		t.Error("no texture decoded from the package")
	}
}

// TestCosmeticDBInPackageMatchesEmbedded checks that the database the app ships
// with is byte-identical to the one in a real install, under every asset name
// the platform publishes it as. That is what lets the Quest build skip
// extraction entirely. Opt in with EVR_DATA_DIR and EVR_EMBEDDED_DB.
func TestCosmeticDBInPackageMatchesEmbedded(t *testing.T) {
	dataDir, embeddedPath := os.Getenv("EVR_DATA_DIR"), os.Getenv("EVR_EMBEDDED_DB")
	if dataDir == "" || embeddedPath == "" {
		t.Skip("set EVR_DATA_DIR and EVR_EMBEDDED_DB to run this")
	}
	p := ParsePlatform(os.Getenv("EVR_PLATFORM"))
	embedded, err := os.ReadFile(embeddedPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range p.CosmeticDBAssetHashes() {
		b, err := ReadPackageAsset(dataDir, p.CosmeticDBTypeHash(), h)
		if err != nil {
			t.Errorf("%s: %v", h, err)
			continue
		}
		if string(b) != string(embedded) {
			t.Errorf("%s differs from the embedded database (%d vs %d bytes)", h, len(b), len(embedded))
		}
	}
}
