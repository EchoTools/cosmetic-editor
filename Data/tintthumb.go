package data

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"math"
)

// Tint thumbnails.
//
// The thumbnail template is the Solar Flare tint's own thumbnail: its first
// colour fills the red region (#9F1213) and its second the yellow one
// (#ECDB10). A tint's thumbnail is that template with the two regions recoloured.
//
// Measured against the game's own tint thumbnails, 76 of 77 decidable tints put
// the colour stored in bytes 0-12 of the tint's data in the red region and
// bytes 12-24 in the yellow region, and they use those colours exactly. The
// generator used to be handed no colours at all (so both regions came out
// white), and it also darkened every recoloured texel to 60%, so even with the
// right colours its thumbnails were far darker than the game's.

var (
	templateFirst  = color.RGBA{0x9F, 0x12, 0x13, 0xFF}
	templateSecond = color.RGBA{0xEC, 0xDB, 0x10, 0xFF}
)

// TintThumbnailColors returns the two colours a tint's thumbnail is drawn in,
// in template order: the red region's first, then the yellow region's.
func TintThumbnailColors(e CosmeticEntry) (first, second color.RGBA, ok bool) {
	ext := e.CEntryExtData
	if len(ext) < 24 {
		return first, second, false
	}
	at := func(off int) color.RGBA {
		c := func(o int) uint8 {
			v := float64(math.Float32frombits(binary.LittleEndian.Uint32(ext[off+o:])))
			return uint8(math.Max(0, math.Min(255, math.Round(v*255))))
		}
		return color.RGBA{c(0), c(4), c(8), 0xFF}
	}
	return at(0), at(12), true
}

// ColorHex formats a colour as the RRGGBB text the editors use.
func ColorHex(c color.RGBA) string { return fmt.Sprintf("%02X%02X%02X", c.R, c.G, c.B) }

// RecolorTintTemplate draws the template in a tint's two colours. Texels close
// to the template's own region colours take the new colour exactly; everything
// else (outlines, shading, background) is kept.
func RecolorTintTemplate(tpl image.Image, first, second color.RGBA) *image.RGBA {
	similar := func(a, b color.RGBA) bool {
		dr := float64(a.R) - float64(b.R)
		dg := float64(a.G) - float64(b.G)
		db := float64(a.B) - float64(b.B)
		return math.Sqrt(dr*dr+dg*dg+db*db) < 20
	}
	b := tpl.Bounds()
	dst := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := tpl.At(x, y).RGBA()
			c := color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), uint8(a >> 8)}
			switch {
			case similar(c, templateFirst):
				c = color.RGBA{first.R, first.G, first.B, c.A}
			case similar(c, templateSecond):
				c = color.RGBA{second.R, second.G, second.B, c.A}
			}
			dst.Set(x, y, c)
		}
	}
	return dst
}
