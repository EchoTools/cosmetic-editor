package texture

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// astcFileHeader is the 16-byte header of a standalone .astc file.
type astcFileHeader struct {
	blockX, blockY int
	width, height  int
}

func readASTCFile(t *testing.T, path string) (astcFileHeader, []byte) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(b) < 16 || binary.LittleEndian.Uint32(b) != 0x5CA1AB13 {
		t.Fatalf("%s is not an .astc file", path)
	}
	dim := func(o int) int { return int(b[o]) | int(b[o+1])<<8 | int(b[o+2])<<16 }
	return astcFileHeader{
		blockX: int(b[4]), blockY: int(b[5]),
		width: dim(7), height: dim(10),
	}, b[16:]
}

func readPNG(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return img
}

// TestDecodeASTCMatchesReference decodes every fixture with the pure-Go decoder
// and compares it against the same block data decoded by ARM's astcenc. ASTC
// decoding is exactly specified, so the bar is bit-exactness, not similarity.
func TestDecodeASTCMatchesReference(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "astc", "*.astc"))
	if err != nil || len(files) == 0 {
		t.Skip("no ASTC fixtures")
	}
	for _, path := range files {
		name := strings.TrimSuffix(filepath.Base(path), ".astc")
		t.Run(name, func(t *testing.T) {
			hdr, blocks := readASTCFile(t, path)
			got, err := DecodeASTC(blocks, hdr.width, hdr.height, hdr.blockX, hdr.blockY)
			if err != nil {
				t.Fatalf("DecodeASTC: %v", err)
			}
			want := readPNG(t, strings.TrimSuffix(path, ".astc")+".ref.png")

			if got.Bounds() != want.Bounds() {
				t.Fatalf("bounds %v, reference %v", got.Bounds(), want.Bounds())
			}
			// Compare straight (non-premultiplied) values. Going through
			// color.RGBA would premultiply both sides and hide alpha errors
			// behind matching colour channels.
			nrgbaAt := func(img image.Image, x, y int) [4]uint8 {
				switch m := img.(type) {
				case *image.NRGBA:
					i := m.PixOffset(x, y)
					return [4]uint8{m.Pix[i], m.Pix[i+1], m.Pix[i+2], m.Pix[i+3]}
				default:
					c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
					return [4]uint8{c.R, c.G, c.B, c.A}
				}
			}

			var diffs, maxErr int
			var firstDiff string
			for y := 0; y < hdr.height; y++ {
				for x := 0; x < hdr.width; x++ {
					g := nrgbaAt(got, x, y)
					w := nrgbaAt(want, x, y)
					d := 0
					for i := 0; i < 4; i++ {
						e := int(g[i]) - int(w[i])
						if e < 0 {
							e = -e
						}
						if e > d {
							d = e
						}
					}
					if d != 0 {
						diffs++
						if d > maxErr {
							maxErr = d
						}
						if firstDiff == "" {
							firstDiff = fmt.Sprintf("at (%d,%d) got rgba%v want rgba%v", x, y, g, w)
						}
					}
				}
			}
			if diffs != 0 {
				t.Errorf("%d of %d texels differ (max channel error %d); first: %s",
					diffs, hdr.width*hdr.height, maxErr, firstDiff)
			}
		})
	}
}

func TestISEBitCount(t *testing.T) {
	// Spot values from the ASTC integer sequence encoding: a plain power-of-two
	// level costs exactly n*bits, while trit and quint levels add their shared
	// block overhead.
	cases := []struct{ n, levels, want int }{
		{1, 2, 1},
		{8, 2, 8},
		{8, 32, 40},
		{5, 3, 8},   // one trit block holds five values in 8 bits
		{3, 5, 7},   // one quint block holds three values in 7 bits
		{10, 3, 16}, // two trit blocks
		{6, 5, 14},  // two quint blocks
	}
	for _, c := range cases {
		if got := iseBitCount(c.n, c.levels); got != c.want {
			t.Errorf("iseBitCount(%d, %d) = %d, want %d", c.n, c.levels, got, c.want)
		}
	}
}

func TestBTQCoversEveryQuantLevel(t *testing.T) {
	for _, levels := range weightLevels {
		bits, trits, quints := btq(levels)
		mul := 1
		if trits == 1 {
			mul = 3
		}
		if quints == 1 {
			mul = 5
		}
		if mul<<uint(bits) != levels {
			t.Errorf("btq(%d) = bits %d trits %d quints %d, which does not reconstruct %d",
				levels, bits, trits, quints, levels)
		}
	}
}

// TestDecodeASTCRejectsShortData guards the bounds check that stops a truncated
// payload reading past the end of the buffer.
func TestDecodeASTCRejectsShortData(t *testing.T) {
	if _, err := DecodeASTC(make([]byte, 16), 64, 64, 4, 4); err == nil {
		t.Error("expected an error for a payload far too short for the extent")
	}
	if _, err := DecodeASTC(make([]byte, 16), 0, 4, 4, 4); err == nil {
		t.Error("expected an error for a zero-width extent")
	}
}
