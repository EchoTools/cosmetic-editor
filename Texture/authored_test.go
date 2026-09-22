package texture

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAuthoredQuestReplacement decodes a texture built by hand for a Quest
// install, rather than one shipped by the game. It is the write path in
// reverse: whatever the editor produces has to look like this, so if the
// header model is wrong in a way the shipped files happen not to expose, this
// is where it shows.
func TestAuthoredQuestReplacement(t *testing.T) {
	dir := filepath.Join("testdata", "quest-authored")
	header, err := os.ReadFile(filepath.Join(dir, "2dfe2e7610506f03"))
	if err != nil {
		t.Skipf("no authored fixture: %v", err)
	}
	payload, err := os.ReadFile(filepath.Join(dir, "2dfe2e7610506f03.gpu"))
	if err != nil {
		t.Skipf("no authored payload: %v", err)
	}

	if len(header) != HeaderSizeQuest {
		t.Errorf("authored header is %d bytes, want the Quest size %d", len(header), HeaderSizeQuest)
	}

	d, err := ParseDescriptor(header, true)
	if err != nil {
		t.Fatalf("ParseDescriptor: %v", err)
	}
	if d.Width != 1024 || d.Height != 1024 || d.MipLevels != 11 {
		t.Errorf("got %dx%d with %d mips, want 1024x1024 with 11", d.Width, d.Height, d.MipLevels)
	}
	if f := d.TextureFormat(); f != 88 || !f.IsASTC() {
		t.Errorf("format = %d (%s), want 88 ASTC_8x8_SRGB", f, f.Name())
	}

	// The declared size, the mip-chain arithmetic and the payload on disk must
	// all agree, or the game rejects the texture.
	if got := d.ExpectedTexelSize(); got != int(d.ResidentSize) {
		t.Errorf("mip chain computes to %d bytes, header declares %d", got, d.ResidentSize)
	}
	if len(payload) != int(d.ResidentSize) {
		t.Errorf("payload is %d bytes, header declares %d", len(payload), d.ResidentSize)
	}

	img, err := Decode(d, payload)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if img.Bounds().Dx() != 1024 || img.Bounds().Dy() != 1024 {
		t.Errorf("decoded %v, want 1024x1024", img.Bounds())
	}

	// A decode that silently failed would come back uniformly magenta, which is
	// the colour ASTC hardware produces for an undecodable block.
	allMagenta := true
	for y := 0; y < 1024 && allMagenta; y += 37 {
		for x := 0; x < 1024; x += 37 {
			i := img.PixOffset(x, y)
			if img.Pix[i] != 255 || img.Pix[i+1] != 0 || img.Pix[i+2] != 255 {
				allMagenta = false
				break
			}
		}
	}
	if allMagenta {
		t.Error("every sampled texel is the error colour, so no block decoded")
	}
}
