package data

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	texture "github.com/EchoTools/cosmetic-editor/Texture"
)

// gradient is a recognisable test picture: red rises left to right, green top
// to bottom.
func gradient(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{uint8(255 * x / max(1, w-1)), uint8(255 * y / max(1, h-1)), 64, 255})
		}
	}
	return img
}

// checkReplacement verifies a built replacement against the header it
// replaces: same shape, correct size, and it decodes to the picture.
func checkReplacement(t *testing.T, orig *texture.Descriptor, rep *QuestReplacement) {
	t.Helper()
	got, err := texture.ParseDescriptor(rep.Header, true)
	if err != nil {
		t.Fatalf("replacement header does not parse: %v", err)
	}
	if len(rep.Header) < texture.HeaderSizeQuest {
		t.Errorf("header is %d bytes, below the Quest size", len(rep.Header))
	}
	if orig != nil {
		same := got.Width == orig.Width && got.Height == orig.Height &&
			got.MipLevels == orig.MipLevels && got.Format == orig.Format &&
			got.ResidentWidth == orig.ResidentWidth && got.ResidentHeight == orig.ResidentHeight &&
			got.ResidentMips == orig.ResidentMips && got.ArraySize == orig.ArraySize &&
			got.FlagA == orig.FlagA && got.FlagB == orig.FlagB && got.FlagC == orig.FlagC
		if !same {
			t.Errorf("replacement header changed the texture's shape:\n  orig %+v\n  got  %+v", *orig, *got)
		}
		if (len(orig.Payload) > 0) != (rep.GPU == nil) {
			t.Errorf("original inline=%v, replacement GPU file present=%v", len(orig.Payload) > 0, rep.GPU != nil)
		}
	}

	payload := got.Payload
	if rep.GPU != nil {
		payload = rep.GPU
	}
	if len(payload) != int(got.ResidentSize) || len(payload) != got.ExpectedTexelSize() {
		t.Errorf("payload %d bytes, header declares %d, mip chain needs %d",
			len(payload), got.ResidentSize, got.ExpectedTexelSize())
	}

	img, err := texture.Decode(got, payload)
	if err != nil {
		t.Fatalf("replacement does not decode: %v", err)
	}
	// The gradient runs red left to right; check the ends are the right way
	// round so a transposed or scrambled mip 0 fails.
	b := img.Bounds()
	if b.Dx() >= 8 {
		left := img.NRGBAAt(b.Min.X, b.Min.Y+b.Dy()/2)
		right := img.NRGBAAt(b.Max.X-1, b.Min.Y+b.Dy()/2)
		if int(right.R)-int(left.R) < 150 {
			t.Errorf("mip 0 red runs %d -> %d, want a strong left-to-right rise", left.R, right.R)
		}
	}
}

func TestBuildQuestTextureStandalone(t *testing.T) {
	rep, err := BuildQuestTexture(nil, gradient(128, 128))
	if err != nil {
		t.Fatal(err)
	}
	if rep.GPU == nil || len(rep.Header) != texture.HeaderSizeQuest {
		t.Errorf("a new texture should be a bare 248-byte header plus a GPU file")
	}
	checkReplacement(t, nil, rep)
}

// TestBuildQuestTextureAgainstRealHeaders replaces textures of every shape in a
// real Quest install: 4x4 and 8x8, inline and separate GPU payload, single and
// multi-mip. Opt in with EVR_QUEST_INSTALL; the install is only read.
func TestBuildQuestTextureAgainstRealHeaders(t *testing.T) {
	dataDir := os.Getenv("EVR_QUEST_INSTALL")
	if dataDir == "" {
		t.Skip("set EVR_QUEST_INSTALL to a Quest data directory to run this")
	}
	defer ClosePackageReader()
	r, err := openPackageReader(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	type shape struct {
		format        uint32
		inline, multi bool
	}
	seen := map[shape]bool{}
	src := packageSource(dataDir)
	metaType := HexToSymbol(PlatformQuest.TextureMetaTypeHash())
	for key := range r.index {
		if key.typ != metaType {
			continue
		}
		hexStr := SymbolToHex(key.file)
		orig, err := LoadTextureHeader(PlatformQuest, src, hexStr)
		if err != nil || !orig.TextureFormat().IsASTC() || orig.ArraySize > 1 {
			continue
		}
		s := shape{orig.Format, len(orig.Payload) > 0, orig.ResidentMips > 1}
		if seen[s] || orig.ResidentWidth > 1024 {
			continue // one of each shape, and keep it quick
		}
		seen[s] = true
		t.Run(filepath.Base(hexStr), func(t *testing.T) {
			rep, err := BuildQuestTexture(orig, gradient(64, 64))
			if err != nil {
				t.Fatalf("%s %dx%d mips=%d inline=%v: %v", orig.TextureFormat().Name(),
					orig.ResidentWidth, orig.ResidentHeight, orig.ResidentMips, s.inline, err)
			}
			checkReplacement(t, orig, rep)
			t.Logf("%s %dx%d mips=%d inline=%v streamed=%v", orig.TextureFormat().Name(),
				orig.ResidentWidth, orig.ResidentHeight, orig.ResidentMips, s.inline, rep.Streamed)
		})
	}
	if len(seen) < 4 {
		t.Errorf("only %d texture shapes found, expected at least 4", len(seen))
	}
}
