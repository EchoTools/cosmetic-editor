package texture

import (
	"encoding/binary"
	"fmt"
)

// Echo VR describes every texture with a fixed header whose real fields begin at
// fieldBase; everything before that is 0xFF fill.  The header is followed, for
// textures whose top mips ship resident rather than streamed, by the texel
// payload inline.
//
// The two platforms differ in exactly one structural way, and it is the reason a
// PC-shaped texture written to a Quest build is rejected:
//
//	PC     header is 0x100 bytes; +0xF8 holds DDS dwPitchOrLinearSize, and the
//	       inline payload is a complete DDS file (148-byte DDS+DX10 header first).
//	Quest  header is 0xF8 bytes with no pitch field, and the inline payload is
//	       raw block data with no container header at all.
//
// Recovered by checking every candidate offset against the DDS header of the
// matching GPU payload, then against mip-chain arithmetic.  On the Quest extract
// residentSize == MipChainSize(resident extent) * arraySize holds for 6464 of
// 6464 descriptors; on PC the same identity holds once the 148-byte DDS header
// is added, for 11587 of 12275 (the remainder are odd uncompressed and cubemap
// layouts, where the DDS header in the payload is authoritative anyway).
const (
	fieldBase = 0xC0

	// HeaderSizePC is the size of a PC texture header, including the trailing
	// pitch and reserved words.
	HeaderSizePC = 0x100
	// HeaderSizeQuest is the size of a Quest texture header.
	HeaderSizeQuest = 0xF8

	// ddsHeaderSize is the DDS magic + DDS_HEADER + DX10 extension that prefixes
	// every PC texel payload.
	ddsHeaderSize = 148
)

// Descriptor is the parsed texture header.
//
// Width/Height/MipLevels describe the texture as authored.  ResidentWidth and
// friends describe the part that actually ships inside the package; for a
// streamed texture the resident part is a small tail of the mip chain (a 4096²
// texture commonly ships 128² and eight mips).  Previewing a texture means
// decoding the resident part, because that is all the file contains.
type Descriptor struct {
	Dimension uint32
	Width     uint32
	Height    uint32
	MipLevels uint32
	ArraySize uint32
	Format    uint32 // DXGI_FORMAT on PC, ETextureFormat on Quest
	FlagA     uint32
	FlagB     uint32
	FlagC     uint32

	ResidentWidth  uint32
	ResidentHeight uint32
	ResidentMips   uint32
	ResidentSize   uint32 // bytes of texel payload, including the DDS header on PC

	Pitch uint32 // PC only: DDS dwPitchOrLinearSize

	// Payload is the inline texel data when the header carried it, else nil.
	// When nil the texels live in the GPU sidecar file instead.
	Payload []byte

	// Quest selects which platform's layout this descriptor uses.  It is set
	// from the type folder the file came from, never inferred from the bytes:
	// the two format enums overlap numerically, so the payload alone cannot
	// say which platform wrote it.
	Quest bool
}

// HeaderSize returns the size of this descriptor's fixed header.
func (d *Descriptor) HeaderSize() int {
	if d.Quest {
		return HeaderSizeQuest
	}
	return HeaderSizePC
}

// IsQuest reports which platform's layout this descriptor was read as.
// ParseDescriptor reads a texture header.  quest selects the header size and how
// the Format field is interpreted; it comes from which type folder the file was
// found in, since the two format enums overlap numerically and the bytes alone
// cannot distinguish them.
func ParseDescriptor(b []byte, quest bool) (*Descriptor, error) {
	hdr := HeaderSizePC
	if quest {
		hdr = HeaderSizeQuest
	}
	if len(b) < hdr {
		return nil, fmt.Errorf("texture header is %d bytes, want at least %d", len(b), hdr)
	}
	u := func(off int) uint32 { return binary.LittleEndian.Uint32(b[fieldBase+off:]) }
	d := &Descriptor{
		Dimension:      u(0x00),
		Width:          u(0x04),
		Height:         u(0x08),
		MipLevels:      u(0x0C),
		ArraySize:      u(0x10),
		Format:         u(0x18),
		FlagA:          u(0x1C),
		FlagB:          u(0x20),
		FlagC:          u(0x24),
		ResidentWidth:  u(0x28),
		ResidentHeight: u(0x2C),
		ResidentMips:   u(0x30),
		ResidentSize:   u(0x34),
		Quest:          quest,
	}
	if !quest {
		d.Pitch = u(0x38)
	}
	if d.Width == 0 || d.Height == 0 {
		return nil, fmt.Errorf("texture header has zero extent %dx%d", d.Width, d.Height)
	}
	if d.MipLevels == 0 {
		d.MipLevels = 1
	}
	if d.ArraySize == 0 {
		d.ArraySize = 1
	}
	if d.ResidentWidth == 0 || d.ResidentHeight == 0 {
		d.ResidentWidth, d.ResidentHeight, d.ResidentMips = d.Width, d.Height, d.MipLevels
	}
	if d.ResidentMips == 0 {
		d.ResidentMips = 1
	}
	if len(b) > hdr {
		d.Payload = b[hdr:]
	}
	return d, nil
}

// Bytes serialises the header back to disk form, preserving the 0xFF fill the
// game expects and appending any inline payload.
func (d *Descriptor) Bytes() []byte {
	hdr := d.HeaderSize()
	b := make([]byte, hdr, hdr+len(d.Payload))
	for i := 0; i < fieldBase; i++ {
		b[i] = 0xFF
	}
	put := func(off int, v uint32) { binary.LittleEndian.PutUint32(b[fieldBase+off:], v) }
	put(0x00, d.Dimension)
	put(0x04, d.Width)
	put(0x08, d.Height)
	put(0x0C, d.MipLevels)
	put(0x10, d.ArraySize)
	put(0x18, d.Format)
	put(0x1C, d.FlagA)
	put(0x20, d.FlagB)
	put(0x24, d.FlagC)
	put(0x28, d.ResidentWidth)
	put(0x2C, d.ResidentHeight)
	put(0x30, d.ResidentMips)
	put(0x34, d.ResidentSize)
	if !d.Quest {
		put(0x38, d.Pitch)
	}
	return append(b, d.Payload...)
}

// TextureFormat folds the platform-specific format field onto the shared
// ETextureFormat enum.
func (d *Descriptor) TextureFormat() ETextureFormat {
	if d.Quest {
		return ETextureFormat(d.Format)
	}
	return dxgiToETextureFormat(DXGIFormat(d.Format))
}

// ExpectedTexelSize returns the byte size the resident texel data should occupy,
// excluding any container header.  This is what a replacement payload must match.
func (d *Descriptor) ExpectedTexelSize() int {
	f := d.TextureFormat()
	if _, ok := f.Info(); !ok {
		return 0
	}
	return f.MipChainSize(int(d.ResidentWidth), int(d.ResidentHeight), int(d.ResidentMips)) * int(d.ArraySize)
}

// Texels returns just the texel bytes of a payload, dropping the DDS container
// header that PC payloads carry and Quest payloads do not.
func (d *Descriptor) Texels(payload []byte) []byte {
	if !d.Quest && len(payload) >= ddsHeaderSize && string(payload[:4]) == "DDS " {
		return payload[ddsHeaderSize:]
	}
	return payload
}
