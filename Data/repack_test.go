package data

import (
	"bytes"
	"image"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
}

// TestRepackQuestInstall runs a full repack against a copy of a real Quest
// install and reads the result back. The copy is taken first, so the source
// install is never written to. Opt in with EVR_QUEST_INSTALL pointing at a clean
// .../rad15/android directory.
func TestRepackQuestInstall(t *testing.T) {
	src := os.Getenv("EVR_QUEST_INSTALL")
	if src == "" {
		t.Skip("set EVR_QUEST_INSTALL to a clean Quest data directory to run this")
	}
	if !HasManifest(src) {
		t.Fatalf("no manifest in %s", src)
	}

	// 1. Work on a copy.
	dataDir := filepath.Join(t.TempDir(), "android")
	defer ClosePackageReader() // or the temp copy cannot be deleted on Windows
	copyFile(t, ManifestPath(src), ManifestPath(dataDir))
	copyFile(t, PackageChunkPath(src, 0), PackageChunkPath(dataDir, 0))
	if got := CountPackageChunks(dataDir); got != 1 {
		t.Fatalf("clean Quest install has %d chunks, want 1", got)
	}

	p := PlatformQuest
	original, err := ReadPackageAsset(dataDir, p.CosmeticDBTypeHash(), p.CosmeticDBAssetHashes()[0])
	if err != nil {
		t.Fatalf("read original database: %v", err)
	}

	// 2. Change one display name and stage the database the way the app does.
	list, err := BytesToCosmeticList(original)
	if err != nil {
		t.Fatal(err)
	}
	const marker = "REPACK TEST NAME"
	entry := &list.CosmeticEntries[0]
	for i := range entry.CEntry.DisplayNameString {
		entry.CEntry.DisplayNameString[i] = 0
	}
	copy(entry.CEntry.DisplayNameString[:], marker)
	modified, err := CosmeticListToBytes(list)
	if err != nil {
		t.Fatal(err)
	}
	staging := filepath.Join(t.TempDir(), "input-quest")
	for _, h := range p.CosmeticDBAssetHashes() {
		path := filepath.Join(staging, p.CosmeticDBTypeHash(), h)
		os.MkdirAll(filepath.Dir(path), 0755)
		if err := os.WriteFile(path, modified, 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Also replace two textures: one with its texels inline in the header and
	// one with a separate GPU payload, the two ways a Quest texture is stored.
	replaced := map[string]bool{}
	{
		r, err := openPackageReader(dataDir)
		if err != nil {
			t.Fatal(err)
		}
		metaType := HexToSymbol(p.TextureMetaTypeHash())
		want := map[bool]bool{true: false, false: false} // inline -> found
		for key := range r.index {
			if key.typ != metaType || (want[true] && want[false]) {
				continue
			}
			h := SymbolToHex(key.file)
			orig, err := LoadTextureHeader(p, packageSource(dataDir), h)
			if err != nil || !orig.TextureFormat().IsASTC() || orig.ArraySize > 1 || orig.ResidentWidth < 16 {
				continue
			}
			inline := len(orig.Payload) > 0
			if want[inline] {
				continue
			}
			rep, err := BuildQuestTexture(orig, gradient(64, 64))
			if err != nil {
				t.Fatalf("build %s: %v", h, err)
			}
			write := func(folder string, b []byte) {
				path := filepath.Join(staging, folder, h)
				os.MkdirAll(filepath.Dir(path), 0755)
				if err := os.WriteFile(path, b, 0644); err != nil {
					t.Fatal(err)
				}
			}
			write(p.TextureMetaTypeHash(), rep.Header)
			if rep.GPU != nil {
				write(p.TextureGPUTypeHash(), rep.GPU)
			}
			want[inline] = true
			replaced[h] = inline
		}
		if len(replaced) != 2 {
			t.Fatalf("found %d replaceable textures, want one inline and one with a GPU payload", len(replaced))
		}
	}

	// 3. Repack.  The app closes the reader before repacking; do the same.
	ClosePackageReader()
	if err := RepackInto(dataDir, staging); err != nil {
		t.Fatalf("RepackInto: %v", err)
	}

	// 4. The new chunk is _1, as on a real headset.
	if got := CountPackageChunks(dataDir); got != 2 {
		t.Errorf("after repack there are %d chunks, want 2 (_0 and the appended _1)", got)
	}

	// 5. Both database names now read back as the modified bytes.
	for _, h := range p.CosmeticDBAssetHashes() {
		b, err := ReadPackageAsset(dataDir, p.CosmeticDBTypeHash(), h)
		if err != nil {
			t.Errorf("read back %s: %v", h, err)
			continue
		}
		if !bytes.Equal(b, modified) {
			t.Errorf("%s did not read back as the repacked database", h)
		}
		if !bytes.Contains(b, []byte(marker)) {
			t.Errorf("%s is missing the edited display name", h)
		}
	}

	// 6. The replaced textures read back out of the package as the gradient.
	for h, inline := range replaced {
		img, err := DecodeTextureAsset(p, packageSource(dataDir), h)
		if err != nil {
			t.Errorf("replaced texture %s (inline=%v) does not decode after repack: %v", h, inline, err)
			continue
		}
		nrgba, ok := img.(*image.NRGBA)
		if !ok {
			continue
		}
		b := nrgba.Bounds()
		left := nrgba.NRGBAAt(b.Min.X, b.Min.Y+b.Dy()/2)
		right := nrgba.NRGBAAt(b.Max.X-1, b.Min.Y+b.Dy()/2)
		if int(right.R)-int(left.R) < 150 {
			t.Errorf("replaced texture %s (inline=%v) did not read back as the replacement", h, inline)
		}
	}

	// 7. Untouched assets still read and decode: the repack must not have
	// damaged the frames it did not modify.
	r, err := openPackageReader(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	metaType := HexToSymbol(p.TextureMetaTypeHash())
	decoded := 0
	for key := range r.index {
		if key.typ != metaType {
			continue
		}
		if _, err := DecodeTextureAsset(p, packageSource(dataDir), SymbolToHex(key.file)); err == nil {
			decoded++
		}
		if decoded == 10 {
			break
		}
	}
	if decoded < 10 {
		t.Errorf("only %d untouched textures decoded after the repack", decoded)
	}
}
