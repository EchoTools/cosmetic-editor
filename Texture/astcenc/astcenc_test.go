package astcenc

import (
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	texture "github.com/EchoTools/cosmetic-editor/Texture"
)

func loadPNG(t *testing.T, name string) image.Image {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// psnr compares two images channel by channel, alpha included.
func psnr(a, b image.Image) float64 {
	var sum float64
	n := 0
	r := a.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			for _, d := range []float64{
				float64(ar>>8) - float64(br>>8), float64(ag>>8) - float64(bg>>8),
				float64(ab>>8) - float64(bb>>8), float64(aa>>8) - float64(ba>>8),
			} {
				sum += d * d
				n++
			}
		}
	}
	mse := sum / float64(n)
	if mse == 0 {
		return math.Inf(1)
	}
	return 10 * math.Log10(255*255/mse)
}

// TestEncodeRoundTrip compresses with the built-in astcenc, decodes with the
// app's own decoder and measures how close the result is to the source. The
// block sizes are the two Echo ships on Quest; the floors are well below what
// astcenc achieves on this image, so a failure means the encode or the decode
// is broken rather than merely lossy.
func TestEncodeRoundTrip(t *testing.T) {
	src := loadPNG(t, "src.png")
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	for _, c := range []struct {
		bw, bh int
		floor  float64
	}{
		{4, 4, 32},
		{8, 8, 24},
	} {
		blocks, err := Encode(src, c.bw, c.bh, false, QualityMedium)
		if err != nil {
			t.Fatalf("%dx%d: Encode: %v", c.bw, c.bh, err)
		}
		want := ((w + c.bw - 1) / c.bw) * ((h + c.bh - 1) / c.bh) * 16
		if len(blocks) != want {
			t.Fatalf("%dx%d: %d bytes, want %d", c.bw, c.bh, len(blocks), want)
		}
		got, err := texture.DecodeASTC(blocks, w, h, c.bw, c.bh)
		if err != nil {
			t.Fatalf("%dx%d: DecodeASTC: %v", c.bw, c.bh, err)
		}
		p := psnr(src, got)
		t.Logf("%dx%d: %d bytes, PSNR %.1f dB", c.bw, c.bh, len(blocks), p)
		if p < c.floor {
			t.Errorf("%dx%d: PSNR %.1f dB is below %.0f dB", c.bw, c.bh, p, c.floor)
		}
	}
}

func TestEncodeRejectsBadInput(t *testing.T) {
	if _, err := Encode(image.NewNRGBA(image.Rect(0, 0, 0, 0)), 4, 4, false, QualityFast); err == nil {
		t.Error("expected an error for an empty image")
	}
	if _, err := Encode(image.NewNRGBA(image.Rect(0, 0, 8, 8)), 7, 3, false, QualityFast); err == nil {
		t.Error("expected an error for a block size ASTC does not define")
	}
}
