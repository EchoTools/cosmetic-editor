package texture

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// TestDecodeFullQuestExtract decodes every texture in a complete Quest extract.
// It is opt-in because the extract is far too large to commit: point
// EVR_QUEST_EXTRACT at the CGTextureResourceAndroid folder to run it.
func TestDecodeFullQuestExtract(t *testing.T) {
	dir := os.Getenv("EVR_QUEST_EXTRACT")
	if dir == "" {
		t.Skip("set EVR_QUEST_EXTRACT to run against a full extract")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	dump := os.Getenv("EVR_DUMP_DIR")
	var decoded, sidecar, failed int
	reasons := map[string]int{}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil || len(b) < HeaderSizeQuest {
			continue
		}
		if len(b) == HeaderSizeQuest {
			sidecar++
			continue
		}
		d, err := ParseDescriptor(b, true)
		if err != nil {
			failed++
			reasons[err.Error()]++
			continue
		}
		img, err := Decode(d, d.Payload)
		if err != nil {
			failed++
			reasons[err.Error()]++
			continue
		}
		decoded++
		if dump != "" && decoded <= 8 {
			f, err := os.Create(filepath.Join(dump, e.Name()+".png"))
			if err == nil {
				png.Encode(f, img)
				f.Close()
			}
		}
	}
	t.Logf("decoded %d, sidecar-only %d, failed %d", decoded, sidecar, failed)
	for r, n := range reasons {
		t.Logf("  %5d x %s", n, r)
	}
}
