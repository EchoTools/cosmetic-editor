package texture

import (
	"fmt"
	"image"
)

// Decode turns a texture's descriptor and texel payload into an image of its
// largest resident mip level.
//
// The payload is the mip chain in descending order, largest level first. That
// ordering is not a guess: the Quest runtime walks it with a cursor that
// advances by one level size per mip while the extent halves, and RAD's own
// name for the level-size array is "reversedmipsizes". Decoding it the other
// way round yields the 1x1 tail mip for level 0, which still renders and so
// hides the mistake.
func Decode(d *Descriptor, payload []byte) (*image.NRGBA, error) {
	if d == nil {
		return nil, fmt.Errorf("nil descriptor")
	}
	texels := d.Texels(payload)
	f := d.TextureFormat()
	info, ok := f.Info()
	if !ok {
		return nil, fmt.Errorf("unsupported texture format %d (%s)", d.Format, f.Name())
	}

	w, h := int(d.ResidentWidth), int(d.ResidentHeight)
	level0 := f.SurfaceSize(w, h)
	if level0 == 0 {
		return nil, fmt.Errorf("cannot size a %dx%d surface in %s", w, h, f.Name())
	}
	if len(texels) < level0 {
		return nil, fmt.Errorf("payload holds %d bytes, mip 0 of a %dx%d %s needs %d",
			len(texels), w, h, f.Name(), level0)
	}

	switch {
	case f.IsASTC():
		return DecodeASTC(texels[:level0], w, h, info.BlockW, info.BlockH)
	case !info.Compressed:
		return decodeUncompressed(texels[:level0], w, h, f)
	default:
		return nil, fmt.Errorf("no decoder for %s", f.Name())
	}
}

// decodeUncompressed handles the plain formats Echo ships alongside the block
// compressed ones.
func decodeUncompressed(data []byte, w, h int, f ETextureFormat) (*image.NRGBA, error) {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	_, _, stride := f.BlockSize()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			src := (y*w + x) * stride
			if src+stride > len(data) {
				return nil, fmt.Errorf("truncated %s surface", f.Name())
			}
			dst := img.PixOffset(x, y)
			switch f.Name() {
			case "R8_UNORM":
				v := data[src]
				img.Pix[dst], img.Pix[dst+1], img.Pix[dst+2], img.Pix[dst+3] = v, v, v, 0xFF
			case "R8G8B8A8_UNORM", "R8G8B8A8_SRGB":
				copy(img.Pix[dst:dst+4], data[src:src+4])
			case "B8G8R8A8_UNORM":
				// Echo stores this channel order literally; swizzling it as RGBA
				// swaps red and blue on every texture that uses it.
				img.Pix[dst+0] = data[src+2]
				img.Pix[dst+1] = data[src+1]
				img.Pix[dst+2] = data[src+0]
				img.Pix[dst+3] = data[src+3]
			default:
				return nil, fmt.Errorf("no decoder for %s", f.Name())
			}
		}
	}
	return img, nil
}

// DecodeLevel decodes one resident mip level; level 0 is the largest. Levels
// are stored largest first, so a level's offset is the sum of the sizes above
// it.
func DecodeLevel(d *Descriptor, payload []byte, level int) (*image.NRGBA, error) {
	if d == nil {
		return nil, fmt.Errorf("nil descriptor")
	}
	if level < 0 || level >= int(d.ResidentMips) {
		return nil, fmt.Errorf("level %d outside the %d resident mips", level, d.ResidentMips)
	}
	texels := d.Texels(payload)
	f := d.TextureFormat()
	info, ok := f.Info()
	if !ok {
		return nil, fmt.Errorf("unsupported texture format %d", d.Format)
	}
	w, h := int(d.ResidentWidth), int(d.ResidentHeight)
	off := 0
	for i := 0; i < level; i++ {
		off += f.SurfaceSize(w, h)
		w, h = max(1, w/2), max(1, h/2)
	}
	size := f.SurfaceSize(w, h)
	if off+size > len(texels) {
		return nil, fmt.Errorf("payload too short for level %d", level)
	}
	data := texels[off : off+size]
	if f.IsASTC() {
		return DecodeASTC(data, w, h, info.BlockW, info.BlockH)
	}
	return decodeUncompressed(data, w, h, f)
}
