package data

import (
	"image"
	"image/color"
	_ "image/png"
	"math"
	"os"
	"testing"
)

func loadTemplate(t *testing.T) image.Image {
	t.Helper()
	f, err := os.Open("template_thumb.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// TestRecolorKeepsExactColours: the game uses a tint's colours exactly, so the
// recoloured regions must come out exactly those colours, not darkened.
func TestRecolorKeepsExactColours(t *testing.T) {
	tpl := loadTemplate(t)
	first := color.RGBA{72, 27, 82, 255}
	second := color.RGBA{197, 184, 34, 255}
	out := RecolorTintTemplate(tpl, first, second)
	var sawFirst, sawSecond bool
	b := tpl.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := out.RGBAAt(x, y)
			if c.R == first.R && c.G == first.G && c.B == first.B {
				sawFirst = true
			}
			if c.R == second.R && c.G == second.G && c.B == second.B {
				sawSecond = true
			}
		}
	}
	if !sawFirst || !sawSecond {
		t.Errorf("recoloured template is missing the exact tint colours (first %v, second %v)", sawFirst, sawSecond)
	}
}

// TestTintThumbnailsMatchTheGame generates a thumbnail for each of the game's
// own tints and compares it with the thumbnail the game ships. Opt in with
// EVR_QUEST_INSTALL and EVR_EMBEDDED_DB.
func TestTintThumbnailsMatchTheGame(t *testing.T) {
	dataDir, db := os.Getenv("EVR_QUEST_INSTALL"), os.Getenv("EVR_EMBEDDED_DB")
	if dataDir == "" || db == "" {
		t.Skip("set EVR_QUEST_INSTALL and EVR_EMBEDDED_DB to run this")
	}
	defer ClosePackageReader()
	tpl := loadTemplate(t)
	b, _ := os.ReadFile(db)
	list, err := BytesToCosmeticList(b)
	if err != nil {
		t.Fatal(err)
	}
	tintSym := int64(ToSymbol("tint"))
	src := packageSource(dataDir)

	var n int
	var sum float64
	worst, worstName := math.Inf(1), ""
	for _, e := range list.CosmeticEntries {
		if e.CEntry.CosmeticTypeSymbol != tintSym || len(e.CEntryExtData) != 24 || e.CEntry.TextureSymbol != -1 {
			continue
		}
		shipped, err := DecodeTextureAsset(PlatformQuest, src, SymbolToHex(e.CEntry.ThumbnailSymbol))
		if err != nil || shipped.Bounds() != tpl.Bounds() {
			continue
		}
		first, second, ok := TintThumbnailColors(e)
		if !ok {
			continue
		}
		gen := RecolorTintTemplate(tpl, first, second)
		var se float64
		px := 0
		bb := gen.Bounds()
		for y := bb.Min.Y; y < bb.Max.Y; y++ {
			for x := bb.Min.X; x < bb.Max.X; x++ {
				g := gen.RGBAAt(x, y)
				r, gg, bl, a := shipped.At(x, y).RGBA()
				if a>>8 < 128 {
					continue // transparent corners carry no colour
				}
				for _, d := range []float64{float64(g.R) - float64(r>>8), float64(g.G) - float64(gg>>8), float64(g.B) - float64(bl>>8)} {
					se += d * d
				}
				px++
			}
		}
		psnr := 10 * math.Log10(255*255/math.Max(se/float64(3*px), 1e-9))
		n++
		sum += psnr
		if psnr < worst {
			worst, worstName = psnr, string(trimNul(e.CEntry.DisplayNameString[:]))
		}
	}
	if n == 0 {
		t.Skip("no comparable tint thumbnails")
	}
	t.Logf("%d tints: generated vs shipped thumbnail mean PSNR %.1f dB, worst %.1f dB (%s)", n, sum/float64(n), worst, worstName)
	if sum/float64(n) < 25 {
		t.Errorf("generated thumbnails are far from the game's (mean PSNR %.1f dB)", sum/float64(n))
	}
}

func trimNul(b []byte) []byte {
	for i, c := range b {
		if c == 0 {
			return b[:i]
		}
	}
	return b
}
