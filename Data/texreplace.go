package data

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"github.com/nfnt/resize"

	texture "github.com/EchoTools/cosmetic-editor/Texture"
	"github.com/EchoTools/cosmetic-editor/Texture/astcenc"
)

// Building Quest texture replacements.
//
// A replacement has to match the texture it replaces in everything but the
// texels: the same ASTC footprint, the same resident extent and the same number
// of resident mips, laid out largest mip first. The header is the original's,
// so the runtime sees exactly the texture it expects.
//
// The previous path encoded every replacement as a single 6x6 mip regardless
// of the original, while the header kept the original's 8x8 or 4x4 format and
// its mip count, so the game read the blocks with the wrong footprint.

// questThumbnailFormat is used for a texture that has no original to copy,
// such as a newly generated thumbnail. It is what the game's own thumbnails
// use: ASTC 4x4 in sRGB, with a single mip.
const questThumbnailFormat texture.ETextureFormat = 74 // ASTC_4x4_SRGB

// QuestReplacement is a built texture ready to stage.
type QuestReplacement struct {
	Header []byte // goes in the CGTextureResourceAndroid folder
	GPU    []byte // goes in the GPU folder; nil when the texels are inline

	// Streamed is set when the original only keeps its smaller mips resident.
	// Only those are replaced; the game still streams the original's larger
	// mips from elsewhere when the texture is seen up close.
	Streamed bool
}

// BuildQuestTexture encodes img to replace the texture described by orig.
// When orig is nil a standalone texture is built at img's size.
func BuildQuestTexture(orig *texture.Descriptor, img image.Image) (*QuestReplacement, error) {
	desc := defaultQuestDescriptor(img)
	inline := false
	if orig != nil {
		d := *orig
		desc = &d
		inline = len(orig.Payload) > 0
	}
	if !desc.Quest {
		return nil, fmt.Errorf("not a Quest texture header")
	}
	if desc.ArraySize > 1 {
		return nil, fmt.Errorf("texture is an array of %d layers; replacing arrays is not supported", desc.ArraySize)
	}

	f := desc.TextureFormat()
	if !f.IsASTC() {
		return nil, fmt.Errorf("texture format %s is not ASTC; only ASTC textures can be replaced on Quest", f.Name())
	}
	bw, bh, _ := f.BlockSize()
	srgb := strings.HasSuffix(f.Name(), "_SRGB")

	w, h := int(desc.ResidentWidth), int(desc.ResidentHeight)
	mips := int(desc.ResidentMips)
	if mips < 1 {
		mips = 1
	}

	// Mip 0 is the picture at the resident size; each further level halves it,
	// never below one texel, and is resampled from the level above.
	var payload []byte
	level := resize.Resize(uint(w), uint(h), img, resize.Lanczos3)
	lw, lh := w, h
	for i := 0; i < mips; i++ {
		if i > 0 {
			lw, lh = max(1, lw/2), max(1, lh/2)
			level = resize.Resize(uint(lw), uint(lh), level, resize.Bilinear)
		}
		blocks, err := astcenc.Encode(level, bw, bh, srgb, astcenc.QualityMedium)
		if err != nil {
			return nil, fmt.Errorf("mip %d: %w", i, err)
		}
		payload = append(payload, blocks...)
	}

	if want := desc.ExpectedTexelSize(); len(payload) != want {
		return nil, fmt.Errorf("encoded %d bytes, the header describes %d", len(payload), want)
	}
	desc.ResidentSize = uint32(len(payload))

	rep := &QuestReplacement{Streamed: desc.Width > desc.ResidentWidth || desc.Height > desc.ResidentHeight}
	if inline {
		desc.Payload = payload
		rep.Header = desc.Bytes()
	} else {
		desc.Payload = nil
		rep.Header = desc.Bytes()
		rep.GPU = payload
	}
	return rep, nil
}

// defaultQuestDescriptor describes a new, fully resident thumbnail-style
// texture at the image's size.
func defaultQuestDescriptor(img image.Image) *texture.Descriptor {
	b := img.Bounds()
	w, h := uint32(b.Dx()), uint32(b.Dy())
	return &texture.Descriptor{
		Dimension: 1, Width: w, Height: h, MipLevels: 1, ArraySize: 1,
		Format: uint32(questThumbnailFormat), FlagA: 1,
		ResidentWidth: w, ResidentHeight: h, ResidentMips: 1,
		Quest: true,
	}
}

// StageQuestTexture builds a replacement for the texture hexStr from img and
// writes it to the staging folder, ready for the next repack. The original
// header comes from a staged copy or the game's package; if the texture does
// not exist yet (a new thumbnail) a standalone one is built.
func StageQuestTexture(state *AppState, hexStr string, img image.Image) (*QuestReplacement, error) {
	return StageQuestTextureLike(state, hexStr, "", img)
}

// StageQuestTextureLike is StageQuestTexture for a texture that may not exist
// in the game yet. If hexStr has no original, templateHex's header is used as
// the shape to build, so a new emote frame matches the frames around it.
func StageQuestTextureLike(state *AppState, hexStr, templateHex string, img image.Image) (*QuestReplacement, error) {
	safe, err := SafeHexFilename(hexStr)
	if err != nil {
		return nil, err
	}
	p := PlatformQuest

	// Read the header from the game's package rather than the staging folder,
	// so replacing a texture twice is sized against the original each time.
	lookup := func(h string) *texture.Descriptor {
		if h == "" {
			return nil
		}
		if d, err := LoadTextureHeader(p, packageSource(state.Settings.EchoVRDataPath), h); err == nil {
			return d
		}
		if d, err := LoadTextureHeader(p, diskSource(state.StagingDir()), h); err == nil {
			return d
		}
		return nil
	}
	orig := lookup(safe)
	if orig == nil && templateHex != "" {
		if tpl, err := SafeHexFilename(templateHex); err == nil {
			orig = lookup(tpl)
		}
	}

	rep, err := BuildQuestTexture(orig, img)
	if err != nil {
		return nil, err
	}

	metaPath := filepath.Join(state.StagingDir(), p.TextureMetaTypeHash(), safe)
	gpuPath := filepath.Join(state.StagingDir(), p.TextureGPUTypeHash(), safe)
	if err := os.MkdirAll(filepath.Dir(metaPath), 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(metaPath, rep.Header, 0644); err != nil {
		return nil, err
	}
	if rep.GPU != nil {
		if err := os.MkdirAll(filepath.Dir(gpuPath), 0755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(gpuPath, rep.GPU, 0644); err != nil {
			return nil, err
		}
	} else {
		// The texels are inline in the header; a GPU file staged by an earlier
		// replacement would now contradict it.
		os.Remove(gpuPath)
	}

	// Drop the cached preview so the replacement is what gets shown.
	if state.Settings.TextureCachePath != "" {
		os.Remove(filepath.Join(state.Settings.TextureCachePath, safe+".png"))
	}
	state.NeedsRepack = true
	return rep, nil
}

// loadImageFile decodes an image from disk.
func loadImageFile(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}
