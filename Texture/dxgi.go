package texture

// DXGIFormat is the Direct3D format enum.  PC (Win10) texture descriptors store
// one of these in their format field, because the Win10 renderer hands the value
// straight to D3D; Quest descriptors store an ETextureFormat instead.
type DXGIFormat uint32

// The DXGI formats Echo VR's PC build actually ships.  Counts come from a census
// of all 3,859 descriptors in a PC extract, and each mapping was checked against
// the pitch/width ratio recorded in the descriptor.
const (
	DXGIR16G16B16A16Float DXGIFormat = 10
	DXGIR11G11B10Float    DXGIFormat = 26
	DXGIR8G8B8A8Unorm     DXGIFormat = 28
	DXGIR8G8B8A8UnormSRGB DXGIFormat = 29
	DXGIR8Unorm           DXGIFormat = 61
	DXGIBC1Unorm          DXGIFormat = 71
	DXGIBC1UnormSRGB      DXGIFormat = 72
	DXGIBC2Unorm          DXGIFormat = 74
	DXGIBC2UnormSRGB      DXGIFormat = 75
	DXGIBC3Unorm          DXGIFormat = 77
	DXGIBC3UnormSRGB      DXGIFormat = 78
	DXGIBC4Unorm          DXGIFormat = 80
	DXGIBC4Snorm          DXGIFormat = 81
	DXGIBC5Unorm          DXGIFormat = 83
	DXGIBC5Snorm          DXGIFormat = 84
	DXGIBC6HUF16          DXGIFormat = 95
	DXGIBC6HSF16          DXGIFormat = 96
	DXGIBC7Unorm          DXGIFormat = 98
	DXGIBC7UnormSRGB      DXGIFormat = 99
	DXGIB8G8R8A8Unorm     DXGIFormat = 87
	DXGIB8G8R8A8UnormSRGB DXGIFormat = 91
)

// dxgiEquiv folds a PC format onto the same ETextureFormat enum the Quest build
// uses, so one decoder serves both platforms.  Sourced from the dxgi_equiv
// column of quest_combat_port's disassembly-derived ETextureFormat table.
var dxgiEquiv = map[DXGIFormat]ETextureFormat{
	DXGIBC1Unorm:          49,
	DXGIBC1UnormSRGB:      50,
	DXGIBC2Unorm:          51,
	DXGIBC2UnormSRGB:      52,
	DXGIBC3Unorm:          53,
	DXGIBC3UnormSRGB:      54,
	DXGIBC4Unorm:          55,
	DXGIBC4Snorm:          56,
	DXGIBC5Unorm:          57,
	DXGIBC5Snorm:          58,
	DXGIBC6HUF16:          59,
	DXGIBC6HSF16:          60,
	DXGIBC7Unorm:          61,
	DXGIBC7UnormSRGB:      62,
	DXGIR8Unorm:           0,
	DXGIR8G8B8A8Unorm:     9,
	DXGIR8G8B8A8UnormSRGB: 13,
	DXGIB8G8R8A8Unorm:     14,
	// No sRGB BGRA exists in ETextureFormat; the linear entry decodes the same
	// bytes and only the transfer curve differs, which preview does not apply.
	DXGIB8G8R8A8UnormSRGB: 14,
	DXGIR16G16B16A16Float: 32,
	DXGIR11G11B10Float:    47,
}

// dxgiToETextureFormat maps a PC format onto the shared enum.  An unmapped
// format returns 0xFFFFFFFF, which no table entry claims, so callers fail
// loudly rather than decoding with the wrong block size.
func dxgiToETextureFormat(f DXGIFormat) ETextureFormat {
	if e, ok := dxgiEquiv[f]; ok {
		return e
	}
	return ETextureFormat(0xFFFFFFFF)
}
