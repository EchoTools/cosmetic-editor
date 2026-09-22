package data

import (
	"image"
	"image/color"
	"testing"
)

// checker is a 2x2 black and white pattern, the case where gamma-space and
// linear-light averaging disagree most.
func checker() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{255, 255, 255, 255})
	img.Set(1, 1, color.NRGBA{255, 255, 255, 255})
	img.Set(1, 0, color.NRGBA{0, 0, 0, 255})
	img.Set(0, 1, color.NRGBA{0, 0, 0, 255})
	return img
}

// TestHalveSRGBAveragesInLinearLight: half white and half black is 50% light,
// which sRGB encodes as 188, not 128. Averaging the stored bytes gives 128 and
// visibly darkens detail in every smaller mip.
func TestHalveSRGBAveragesInLinearLight(t *testing.T) {
	got := toWorking(checker(), true).halve().toNRGBA(true).NRGBAAt(0, 0)
	if got.R < 186 || got.R > 190 {
		t.Errorf("sRGB average of black and white = %d, want about 188", got.R)
	}
}

// TestHalveUNORMAveragesStoredValues: UNORM data (normal maps, masks) is
// already linear, so the plain average is right.
func TestHalveUNORMAveragesStoredValues(t *testing.T) {
	got := toWorking(checker(), false).halve().toNRGBA(false).NRGBAAt(0, 0)
	if got.R < 127 || got.R > 128 {
		t.Errorf("UNORM average of black and white = %d, want 127 or 128", got.R)
	}
}

// TestHalveIgnoresColourOfTransparentTexels: a fully transparent texel's colour
// must not tint its opaque neighbours. Averaging straight colour would pull the
// red toward the black of the invisible texels.
func TestHalveIgnoresColourOfTransparentTexels(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{255, 0, 0, 255})
	img.Set(1, 0, color.NRGBA{0, 0, 0, 0})
	img.Set(0, 1, color.NRGBA{0, 0, 0, 0})
	img.Set(1, 1, color.NRGBA{0, 0, 0, 0})
	for _, srgb := range []bool{true, false} {
		got := toWorking(img, srgb).halve().toNRGBA(srgb).NRGBAAt(0, 0)
		if got.R < 254 || got.G != 0 || got.B != 0 {
			t.Errorf("srgb=%v: colour %v, want pure red with the transparent texels ignored", srgb, got)
		}
		if got.A < 63 || got.A > 64 {
			t.Errorf("srgb=%v: alpha %d, want a quarter", srgb, got.A)
		}
	}
}

// TestHalveOddAndThinSurfaces covers mip chains of non-square and odd extents,
// which the game has (896x368, 13184x8).
func TestHalveOddAndThinSurfaces(t *testing.T) {
	for _, c := range []struct{ w, h, ww, wh int }{{5, 3, 2, 1}, {1, 8, 1, 4}, {8, 1, 4, 1}, {1, 1, 1, 1}} {
		img := image.NewNRGBA(image.Rect(0, 0, c.w, c.h))
		got := toWorking(img, true).halve()
		if got.w != c.ww || got.h != c.wh {
			t.Errorf("%dx%d halved to %dx%d, want %dx%d", c.w, c.h, got.w, got.h, c.ww, c.wh)
		}
	}
}

// TestColourSpaceRoundTrip checks the working space loses nothing on the way
// in and out, so an image that is not resized comes back byte for byte.
func TestColourSpaceRoundTrip(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 256, 1))
	for x := 0; x < 256; x++ {
		img.Set(x, 0, color.NRGBA{uint8(x), uint8(255 - x), uint8(x / 2), 255})
	}
	for _, srgb := range []bool{true, false} {
		out := toWorking(img, srgb).toNRGBA(srgb)
		for i := range img.Pix {
			if out.Pix[i] != img.Pix[i] {
				t.Fatalf("srgb=%v: byte %d changed %d -> %d", srgb, i, img.Pix[i], out.Pix[i])
			}
		}
	}
}
