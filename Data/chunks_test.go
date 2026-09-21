package data

import (
	"os"
	"path/filepath"
	"testing"
)

// makeChunks creates n package chunks in a temporary data directory.
func makeChunks(t *testing.T, n int) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "packages"), 0755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		if err := os.WriteFile(PackageChunkPath(dir, i), []byte("chunk"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCountPackageChunks(t *testing.T) {
	for _, n := range []int{0, 1, 3, 5} {
		dir := makeChunks(t, n)
		if got := CountPackageChunks(dir); got != n {
			t.Errorf("CountPackageChunks with %d chunks = %d", n, got)
		}
	}
}

// TestStockPackageChunks pins the counts a clean install ships with. These
// decide which chunk a repack appends: _3 on PC, _1 on Quest.
func TestStockPackageChunks(t *testing.T) {
	if got := PlatformPC.StockPackageChunks(); got != 3 {
		t.Errorf("PC ships %d chunks, want 3 (so the first repack appends _3)", got)
	}
	if got := PlatformQuest.StockPackageChunks(); got != 1 {
		t.Errorf("Quest ships %d chunks, want 1 (so the first repack appends _1)", got)
	}
}

// TestStockChunkCountPrefersTheRecord checks that a recorded count wins over
// the platform default. An install that has already been repacked has more
// chunks than it shipped with, so the default would delete a stock chunk.
func TestStockChunkCountPrefersTheRecord(t *testing.T) {
	bak := filepath.Join(t.TempDir(), "manifest.bak")
	if err := os.WriteFile(bak, []byte("manifest"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := RecordStockChunkCount(bak, 4); err != nil {
		t.Fatal(err)
	}
	if got := StockChunkCount(bak, PlatformQuest); got != 4 {
		t.Errorf("StockChunkCount = %d, want the recorded 4 rather than the Quest default", got)
	}
}

func TestStockChunkCountFallsBackPerPlatform(t *testing.T) {
	bak := filepath.Join(t.TempDir(), "absent.bak")
	if got := StockChunkCount(bak, PlatformQuest); got != 1 {
		t.Errorf("Quest fallback = %d, want 1", got)
	}
	if got := StockChunkCount(bak, PlatformPC); got != 3 {
		t.Errorf("PC fallback = %d, want 3", got)
	}

	// A corrupt record must not be trusted into deleting stock chunks.
	bad := filepath.Join(t.TempDir(), "bad.bak")
	if err := os.WriteFile(bad+chunkCountSuffix, []byte("not a number"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := StockChunkCount(bad, PlatformQuest); got != 1 {
		t.Errorf("corrupt record gave %d, want the Quest default 1", got)
	}
}

// TestRevertRemovesOnlyAppendedChunks walks the revert arithmetic for both
// platforms: everything from the stock count upward goes, everything below it
// stays.
func TestRevertRemovesOnlyAppendedChunks(t *testing.T) {
	cases := []struct {
		name            string
		platform        Platform
		stock, repacked int
	}{
		{"quest first repack", PlatformQuest, 1, 2},
		{"pc first repack", PlatformPC, 3, 4},
		{"pc second repack", PlatformPC, 3, 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := makeChunks(t, c.repacked)
			bak := filepath.Join(dir, "manifest.bak")
			if err := RecordStockChunkCount(bak, c.stock); err != nil {
				t.Fatal(err)
			}
			for n := StockChunkCount(bak, c.platform); ; n++ {
				p := PackageChunkPath(dir, n)
				if _, err := os.Stat(p); err != nil {
					break
				}
				os.Remove(p)
			}
			if got := CountPackageChunks(dir); got != c.stock {
				t.Errorf("after revert there are %d chunks, want %d", got, c.stock)
			}
		})
	}
}
