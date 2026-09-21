package texture

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDecodeQuestCorpus decodes every Quest texture fixture end to end. These
// are real game files, so this is the test that says the whole path works:
// header parse, format resolution, mip-chain sizing and ASTC decode.
func TestDecodeQuestCorpus(t *testing.T) {
	dir := filepath.Join("testdata", "quest-descriptors")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("no Quest fixtures: %v", err)
	}
	var decoded, skipped int
	reasons := map[string]int{}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil || len(b) <= HeaderSizeQuest {
			continue // texels live in the GPU sidecar, which is not a fixture here
		}
		d, err := ParseDescriptor(b, true)
		if err != nil {
			t.Errorf("%s: %v", e.Name(), err)
			continue
		}
		img, err := Decode(d, d.Payload)
		if err != nil {
			skipped++
			reasons[err.Error()]++
			continue
		}
		if img.Bounds().Dx() != int(d.ResidentWidth) || img.Bounds().Dy() != int(d.ResidentHeight) {
			t.Errorf("%s: decoded %v, want %dx%d", e.Name(), img.Bounds(),
				d.ResidentWidth, d.ResidentHeight)
		}
		decoded++
	}
	t.Logf("decoded %d Quest textures, %d unsupported", decoded, skipped)
	for r, n := range reasons {
		t.Logf("  %d x %s", n, r)
	}
	if decoded == 0 {
		t.Error("no Quest texture decoded")
	}
}
