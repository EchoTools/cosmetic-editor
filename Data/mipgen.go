package data

import (
	"image"
	"image/draw"
	"math"

	"github.com/nfnt/resize"
)

// Mip generation in the texture's own colour space.
//
// Checked against the game's own textures by rebuilding mip 1 from mip 0 both
// ways and comparing with what shipped. For sRGB textures the shipped mip 1 is
// the linear-light average in 1,431 of 1,492 decidable cases; for UNORM
// textures it is the plain average of the stored values in 3,456 of 3,679.
// That is the correct rule: averaging gamma-encoded colour darkens detail as it
// shrinks, while UNORM data (normal maps, masks) is already linear and must not
// be gamma-converted.
//
// Colour is also premultiplied by alpha while it is filtered, so the colour of
// fully transparent texels cannot bleed into the edges of the opaque ones.

// working is an image in the space it is filtered in: premultiplied RGBA,
// with colour linear for sRGB textures.
type working struct {
	w, h int
	px   []float64 // 4 per texel: premultiplied R, G, B, then A, all 0..1
}

var srgbToLinearLUT = func() (t [256]float64) {
	for i := range t {
		v := float64(i) / 255
		if v <= 0.04045 {
			t[i] = v / 12.92
		} else {
			t[i] = math.Pow((v+0.055)/1.055, 2.4)
		}
	}
	return
}()

func linearToSRGB(v float64) float64 {
	if v <= 0.0031308 {
		return v * 12.92
	}
	return 1.055*math.Pow(v, 1/2.4) - 0.055
}

// toWorking converts an image into the filtering space.
func toWorking(img image.Image, srgb bool) *working {
	b := img.Bounds()
	src := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(src, src.Bounds(), img, b.Min, draw.Src)

	wk := &working{w: b.Dx(), h: b.Dy(), px: make([]float64, 4*b.Dx()*b.Dy())}
	for i := 0; i < wk.w*wk.h; i++ {
		p := src.Pix[4*i : 4*i+4]
		a := float64(p[3]) / 255
		for c := 0; c < 3; c++ {
			v := float64(p[c]) / 255
			if srgb {
				v = srgbToLinearLUT[p[c]]
			}
			wk.px[4*i+c] = v * a
		}
		wk.px[4*i+3] = a
	}
	return wk
}

// toNRGBA converts back to straight-alpha 8-bit, re-encoding sRGB colour.
func (wk *working) toNRGBA(srgb bool) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, wk.w, wk.h))
	for i := 0; i < wk.w*wk.h; i++ {
		a := wk.px[4*i+3]
		for c := 0; c < 3; c++ {
			v := 0.0
			if a > 0 {
				v = wk.px[4*i+c] / a
			}
			if srgb {
				v = linearToSRGB(v)
			}
			out.Pix[4*i+c] = to8(v)
		}
		out.Pix[4*i+3] = to8(a)
	}
	return out
}

func to8(v float64) uint8 {
	return uint8(math.Max(0, math.Min(255, math.Round(v*255))))
}

// halve box-filters 2x2 texel blocks into the next mip. An odd edge row or
// column is averaged with itself, and a side already at one texel stays one.
func (wk *working) halve() *working {
	nw, nh := max(1, wk.w/2), max(1, wk.h/2)
	out := &working{w: nw, h: nh, px: make([]float64, 4*nw*nh)}
	for y := 0; y < nh; y++ {
		y0, y1 := min(2*y, wk.h-1), min(2*y+1, wk.h-1)
		for x := 0; x < nw; x++ {
			x0, x1 := min(2*x, wk.w-1), min(2*x+1, wk.w-1)
			for c := 0; c < 4; c++ {
				out.px[4*(y*nw+x)+c] = (wk.px[4*(y0*wk.w+x0)+c] + wk.px[4*(y0*wk.w+x1)+c] +
					wk.px[4*(y1*wk.w+x0)+c] + wk.px[4*(y1*wk.w+x1)+c]) / 4
			}
		}
	}
	return out
}

// resizeWorking resamples to an exact size with Lanczos3, in the filtering
// space. It goes through a 16-bit premultiplied image so the resize library
// does the filtering; 16 bits holds linear-light colour without banding.
func resizeWorking(wk *working, w, h int) *working {
	if wk.w == w && wk.h == h {
		return wk
	}
	src := image.NewRGBA64(image.Rect(0, 0, wk.w, wk.h))
	for i := 0; i < wk.w*wk.h; i++ {
		for c := 0; c < 4; c++ {
			v := uint16(math.Max(0, math.Min(65535, math.Round(wk.px[4*i+c]*65535))))
			src.Pix[8*i+2*c] = uint8(v >> 8)
			src.Pix[8*i+2*c+1] = uint8(v)
		}
	}
	dst, ok := resize.Resize(uint(w), uint(h), src, resize.Lanczos3).(*image.RGBA64)
	if !ok {
		conv := image.NewRGBA64(image.Rect(0, 0, w, h))
		draw.Draw(conv, conv.Bounds(), resize.Resize(uint(w), uint(h), src, resize.Lanczos3), image.Point{}, draw.Src)
		dst = conv
	}
	out := &working{w: w, h: h, px: make([]float64, 4*w*h)}
	for i := 0; i < w*h; i++ {
		for c := 0; c < 4; c++ {
			o := dst.PixOffset(i%w, i/w) + 2*c
			out.px[4*i+c] = float64(uint16(dst.Pix[o])<<8|uint16(dst.Pix[o+1])) / 65535
		}
		// Lanczos rings; a premultiplied colour may not exceed its alpha.
		for c := 0; c < 3; c++ {
			out.px[4*i+c] = math.Min(out.px[4*i+c], out.px[4*i+3])
		}
	}
	return out
}
