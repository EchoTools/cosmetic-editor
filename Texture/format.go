package texture

// ETextureFormat is Echo VR's own texture format enum, stored at +0x0C of every
// 256-byte texture descriptor.  On PC builds that field happens to hold a DXGI
// format number instead, because the Win10 renderer passes it straight to D3D;
// on Quest it holds a value from this table, which the Vulkan renderer maps to
// a VkFormat.  Reading a Quest descriptor as DXGI is what makes ASTC textures
// decode as garbage.
//
// Derived from quest_combat_port/data/etextureformat_table.tsv, which was built
// from libr15.so disassembly.
type ETextureFormat uint32

// formatInfo describes one entry of the enum.
type formatInfo struct {
	Name          string
	BlockW        int
	BlockH        int
	BytesPerBlock int
	Compressed    bool
}

var formatTable = map[ETextureFormat]formatInfo{
	0:   {"R8_UNORM", 1, 1, 1, false},
	5:   {"R8G8_UNORM", 1, 1, 2, false},
	9:   {"R8G8B8A8_UNORM", 1, 1, 4, false},
	13:  {"R8G8B8A8_SRGB", 1, 1, 4, false},
	14:  {"B8G8R8A8_UNORM", 1, 1, 4, false},
	32:  {"R16G16B16A16_SFLOAT", 1, 1, 8, false},
	35:  {"R32_SFLOAT", 1, 1, 4, false},
	44:  {"R32G32B32A32_SFLOAT", 1, 1, 16, false},
	45:  {"A2R10G10B10_UNORM_PACK32", 1, 1, 4, false},
	46:  {"A2R10G10B10_UINT_PACK32", 1, 1, 4, false},
	47:  {"B10G11R11_UFLOAT_PACK32", 1, 1, 4, false},
	48:  {"E5B9G9R9_UFLOAT_PACK32", 1, 1, 4, false},
	49:  {"BC1_RGBA_UNORM", 4, 4, 8, true},
	50:  {"BC1_RGBA_SRGB", 4, 4, 8, true},
	51:  {"BC2_UNORM", 4, 4, 16, true},
	52:  {"BC2_SRGB", 4, 4, 16, true},
	53:  {"BC3_UNORM", 4, 4, 16, true},
	54:  {"BC3_SRGB", 4, 4, 16, true},
	55:  {"BC4_UNORM", 4, 4, 8, true},
	56:  {"BC4_SNORM", 4, 4, 8, true},
	57:  {"BC5_UNORM", 4, 4, 16, true},
	58:  {"BC5_SNORM", 4, 4, 16, true},
	59:  {"BC6H_UFLOAT", 4, 4, 16, true},
	60:  {"BC6H_SFLOAT", 4, 4, 16, true},
	61:  {"BC7_UNORM", 4, 4, 16, true},
	62:  {"BC7_SRGB", 4, 4, 16, true},
	63:  {"ETC2_R8G8B8_UNORM", 4, 4, 8, true},
	64:  {"ETC2_R8G8B8_SRGB", 4, 4, 8, true},
	65:  {"ETC2_R8G8B8A1_UNORM", 4, 4, 8, true},
	66:  {"ETC2_R8G8B8A1_SRGB", 4, 4, 8, true},
	67:  {"ETC2_R8G8B8A8_UNORM", 4, 4, 16, true},
	68:  {"ETC2_R8G8B8A8_SRGB", 4, 4, 16, true},
	69:  {"EAC_R11_UNORM", 4, 4, 8, true},
	70:  {"EAC_R11_SNORM", 4, 4, 8, true},
	71:  {"EAC_R11G11_UNORM", 4, 4, 16, true},
	72:  {"EAC_R11G11_SNORM", 4, 4, 16, true},
	73:  {"ASTC_4x4_UNORM", 4, 4, 16, true},
	74:  {"ASTC_4x4_SRGB", 4, 4, 16, true},
	75:  {"ASTC_5x4_UNORM", 5, 4, 16, true},
	76:  {"ASTC_5x4_SRGB", 5, 4, 16, true},
	77:  {"ASTC_5x5_UNORM", 5, 5, 16, true},
	78:  {"ASTC_5x5_SRGB", 5, 5, 16, true},
	79:  {"ASTC_6x5_UNORM", 6, 5, 16, true},
	80:  {"ASTC_6x5_SRGB", 6, 5, 16, true},
	81:  {"ASTC_6x6_UNORM", 6, 6, 16, true},
	82:  {"ASTC_6x6_SRGB", 6, 6, 16, true},
	83:  {"ASTC_8x5_UNORM", 8, 5, 16, true},
	84:  {"ASTC_8x5_SRGB", 8, 5, 16, true},
	85:  {"ASTC_8x6_UNORM", 8, 6, 16, true},
	86:  {"ASTC_8x6_SRGB", 8, 6, 16, true},
	87:  {"ASTC_8x8_UNORM", 8, 8, 16, true},
	88:  {"ASTC_8x8_SRGB", 8, 8, 16, true},
	89:  {"ASTC_10x5_UNORM", 10, 5, 16, true},
	90:  {"ASTC_10x5_SRGB", 10, 5, 16, true},
	91:  {"ASTC_10x6_UNORM", 10, 6, 16, true},
	92:  {"ASTC_10x6_SRGB", 10, 6, 16, true},
	93:  {"ASTC_10x8_UNORM", 10, 8, 16, true},
	94:  {"ASTC_10x8_SRGB", 10, 8, 16, true},
	95:  {"ASTC_10x10_UNORM", 10, 10, 16, true},
	96:  {"ASTC_10x10_SRGB", 10, 10, 16, true},
	97:  {"ASTC_12x10_UNORM", 12, 10, 16, true},
	98:  {"ASTC_12x10_SRGB", 12, 10, 16, true},
	99:  {"ASTC_12x12_UNORM", 12, 12, 16, true},
	100: {"ASTC_12x12_SRGB", 12, 12, 16, true},
	101: {"D16_UNORM", 1, 1, 2, false},
	102: {"D24_UNORM_S8", 1, 1, 4, false},
	103: {"D32_SFLOAT", 1, 1, 4, false},
	104: {"D32_SFLOAT_S8", 1, 1, 8, false},
}

// ASTC enum range.  Every ASTC format is contiguous in the enum, which makes the
// "is this a Quest texture" test a range check rather than a list.
const (
	FormatASTCFirst ETextureFormat = 73  // ASTC_4x4_UNORM
	FormatASTCLast  ETextureFormat = 100 // ASTC_12x12_SRGB
)

// Info returns the table entry for a format, and whether it is known.
func (f ETextureFormat) Info() (formatInfo, bool) {
	fi, ok := formatTable[f]
	return fi, ok
}

// Name returns the format's symbolic name, or a hex placeholder when unknown.
func (f ETextureFormat) Name() string {
	if fi, ok := formatTable[f]; ok {
		return fi.Name
	}
	return "UNKNOWN"
}

// IsASTC reports whether this format is one of the ASTC block formats.
func (f ETextureFormat) IsASTC() bool {
	return f >= FormatASTCFirst && f <= FormatASTCLast
}

// BlockSize returns the footprint of one compressed block in texels and the
// number of bytes that block occupies.  Uncompressed formats report a 1x1
// block whose size is the pixel stride.
func (f ETextureFormat) BlockSize() (w, h, bytes int) {
	if fi, ok := formatTable[f]; ok {
		return fi.BlockW, fi.BlockH, fi.BytesPerBlock
	}
	return 0, 0, 0
}

// SurfaceSize returns the byte size of a single mip level of the given
// dimensions, rounding up to whole blocks the way every block codec requires.
func (f ETextureFormat) SurfaceSize(width, height int) int {
	bw, bh, bpb := f.BlockSize()
	if bw == 0 || bh == 0 {
		return 0
	}
	return ((width + bw - 1) / bw) * ((height + bh - 1) / bh) * bpb
}

// MipChainSize returns the total byte size of a full mip chain, which is what a
// Quest texture payload actually contains.  Levels stop at 1x1; dimensions are
// halved with a floor of 1 rather than truncating to zero.
func (f ETextureFormat) MipChainSize(width, height, levels int) int {
	total := 0
	w, h := width, height
	for i := 0; i < levels; i++ {
		total += f.SurfaceSize(w, h)
		if w == 1 && h == 1 {
			break
		}
		if w > 1 {
			w /= 2
		}
		if h > 1 {
			h /= 2
		}
	}
	return total
}
