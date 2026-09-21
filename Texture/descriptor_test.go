package texture

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestDescriptorRoundTripPC(t *testing.T) {
	d := &Descriptor{
		Dimension: 1, Width: 1024, Height: 480, MipLevels: 1, ArraySize: 1,
		Format:        uint32(DXGIBC5Unorm),
		ResidentWidth: 1024, ResidentHeight: 480, ResidentMips: 1,
		ResidentSize: 491668, Pitch: 491520,
	}
	b := d.Bytes()
	if len(b) != HeaderSizePC {
		t.Fatalf("PC header = %d bytes, want %d", len(b), HeaderSizePC)
	}
	for i := 0; i < fieldBase; i++ {
		if b[i] != 0xFF {
			t.Fatalf("fill byte %d = %#x, want 0xFF", i, b[i])
		}
	}
	got, err := ParseDescriptor(b, false)
	if err != nil {
		t.Fatalf("ParseDescriptor: %v", err)
	}
	if got.Width != 1024 || got.Height != 480 || got.Pitch != 491520 {
		t.Errorf("got %dx%d pitch=%d, want 1024x480 pitch=491520", got.Width, got.Height, got.Pitch)
	}
	if got.TextureFormat() != 57 { // BC5_UNORM in the shared enum
		t.Errorf("format folded to %d, want 57", got.TextureFormat())
	}
}

func TestDescriptorRoundTripQuest(t *testing.T) {
	d := &Descriptor{
		Dimension: 1, Width: 128, Height: 128, MipLevels: 1, ArraySize: 1,
		Format:        74, // ASTC_4x4_SRGB
		ResidentWidth: 128, ResidentHeight: 128, ResidentMips: 1, ResidentSize: 16384,
		Quest: true,
	}
	b := d.Bytes()
	if len(b) != HeaderSizeQuest {
		t.Fatalf("Quest header = %d bytes, want %d", len(b), HeaderSizeQuest)
	}
	got, err := ParseDescriptor(b, true)
	if err != nil {
		t.Fatalf("ParseDescriptor: %v", err)
	}
	if !got.TextureFormat().IsASTC() {
		t.Errorf("format %d should be ASTC", got.TextureFormat())
	}
	// 128x128 at 4x4, one mip: 32*32 blocks * 16 bytes.
	if got.ExpectedTexelSize() != 16384 {
		t.Errorf("ExpectedTexelSize = %d, want 16384", got.ExpectedTexelSize())
	}
}

// TestPlatformHeadersDifferInSize is the structural difference that makes a
// PC-shaped texture unloadable on Quest.
func TestPlatformHeadersDifferInSize(t *testing.T) {
	if HeaderSizePC == HeaderSizeQuest {
		t.Fatal("PC and Quest texture headers must not be the same size")
	}
	if HeaderSizePC-HeaderSizeQuest != 8 {
		t.Errorf("PC header is %d bytes larger than Quest, want 8 (pitch + reserved)",
			HeaderSizePC-HeaderSizeQuest)
	}
}

// corpusCheck validates every descriptor in a real extract: the inline payload
// length must equal the declared resident size, and that size must equal the
// mip-chain arithmetic for the declared format and resident extent.
func corpusCheck(t *testing.T, dir string, quest bool, headerSize int, extra int) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("no extract at %s: %v", dir, err)
	}
	limit := len(entries)
	if testing.Short() && limit > 400 {
		limit = 400
	}
	checked, chainOK, inlineOK, inline := 0, 0, 0, 0
	for _, e := range entries[:limit] {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil || len(b) < headerSize {
			continue
		}
		d, err := ParseDescriptor(b, quest)
		if err != nil {
			continue
		}
		checked++
		if len(b) > headerSize {
			inline++
			if len(b)-headerSize == int(d.ResidentSize) {
				inlineOK++
			}
		}
		if want := d.ExpectedTexelSize(); want > 0 && want+extra == int(d.ResidentSize) {
			chainOK++
		}
	}
	if checked == 0 {
		t.Skip("extract present but empty")
	}
	t.Logf("%d descriptors: %d/%d inline payloads sized correctly, %d/%d mip-chain arithmetic agrees",
		checked, inlineOK, inline, chainOK, checked)
	if inline > 0 && inlineOK != inline {
		t.Errorf("%d of %d inline payloads did not match the declared resident size", inline-inlineOK, inline)
	}
	if quest && chainOK != checked {
		t.Errorf("Quest mip-chain arithmetic failed for %d of %d descriptors", checked-chainOK, checked)
	}
}

func TestQuestCorpus(t *testing.T) {
	corpusCheck(t, filepath.Join("testdata", "quest-descriptors"), true, HeaderSizeQuest, 0)
}

func TestPCCorpusAgainstDDSHeaders(t *testing.T) {
	metaDir := filepath.Join("..", "Settings", "pcvr-extracted", "4a4c32c49300b8a0")
	gpuDir := filepath.Join("..", "Settings", "pcvr-extracted", "beac1969cb7b8861")
	entries, err := os.ReadDir(metaDir)
	if err != nil {
		t.Skipf("no PC extract: %v", err)
	}
	limit := len(entries)
	if testing.Short() && limit > 400 {
		limit = 400
	}
	checked, bad := 0, 0
	for _, e := range entries[:limit] {
		mb, err := os.ReadFile(filepath.Join(metaDir, e.Name()))
		if err != nil || len(mb) != HeaderSizePC {
			continue
		}
		gb, err := os.ReadFile(filepath.Join(gpuDir, e.Name()))
		if err != nil || len(gb) < ddsHeaderSize || string(gb[:4]) != "DDS " {
			continue
		}
		d, err := ParseDescriptor(mb, false)
		if err != nil {
			t.Errorf("%s: %v", e.Name(), err)
			continue
		}
		checked++
		if d.Width != binary.LittleEndian.Uint32(gb[16:]) ||
			d.Height != binary.LittleEndian.Uint32(gb[12:]) ||
			d.MipLevels != binary.LittleEndian.Uint32(gb[28:]) ||
			d.Pitch != binary.LittleEndian.Uint32(gb[20:]) ||
			d.ResidentSize != uint32(len(gb)) {
			bad++
		}
	}
	if checked == 0 {
		t.Skip("extract present but held no descriptor/payload pairs")
	}
	if bad != 0 {
		t.Errorf("%d of %d PC descriptors disagreed with their DDS header", bad, checked)
	}
	t.Logf("validated %d PC descriptor/payload pairs against their DDS headers", checked)
}
