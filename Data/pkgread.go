package data

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/EchoTools/cosmetic-editor/EvrFile/manifest"
)

// Reading single assets straight out of the game's package.
//
// Extracting a whole package is not an option on a headset: it is several
// gigabytes. The editor only ever needs a handful of assets at a time (the
// texture it is previewing, or the header of the texture it is about to
// replace), so it reads those one at a time. Each read decompresses only the
// frame the asset lives in.

type assetKey struct{ typ, file int64 }

// packageReader keeps one install's manifest and package open between reads.
// Parsing the manifest means decompressing it, so doing that per asset would
// make every preview pay for the whole index.
type packageReader struct {
	mu      sync.Mutex
	dataDir string
	stamp   time.Time // manifest mtime when opened; a repack changes it
	pkg     *manifest.Package
	index   map[assetKey]manifest.FrameContent
}

var (
	readerMu sync.Mutex
	reader   *packageReader
)

// openPackageReader returns a reader for dataDir, reopening it if the manifest
// has changed since it was opened, as it does after a repack.
func openPackageReader(dataDir string) (*packageReader, error) {
	mpath := ManifestPath(dataDir)
	info, err := os.Stat(mpath)
	if err != nil {
		return nil, fmt.Errorf("no package manifest in %s: %w", dataDir, err)
	}

	readerMu.Lock()
	defer readerMu.Unlock()
	if reader != nil && reader.dataDir == dataDir && reader.stamp.Equal(info.ModTime()) {
		return reader, nil
	}
	if reader != nil && reader.pkg != nil {
		reader.pkg.Close()
		reader = nil
	}

	m, err := manifest.ReadFile(mpath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	pkg, err := manifest.OpenPackage(m, filepath.Join(dataDir, "packages", PackageName))
	if err != nil {
		return nil, fmt.Errorf("open package: %w", err)
	}
	idx := make(map[assetKey]manifest.FrameContent, len(m.FrameContents))
	for _, fc := range m.FrameContents {
		idx[assetKey{fc.TypeSymbol, fc.FileSymbol}] = fc
	}
	reader = &packageReader{dataDir: dataDir, stamp: info.ModTime(), pkg: pkg, index: idx}
	return reader, nil
}

// ReadPackageAsset returns one asset's bytes from the install at dataDir.
// typeHex is the resource type folder hash and fileHex the asset hash.
func ReadPackageAsset(dataDir, typeHex, fileHex string) ([]byte, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("no game data directory set")
	}
	typ, file := HexToSymbol(typeHex), HexToSymbol(fileHex)
	if typ == -1 || file == -1 {
		return nil, fmt.Errorf("invalid asset key %s/%s", typeHex, fileHex)
	}

	r, err := openPackageReader(dataDir)
	if err != nil {
		return nil, err
	}
	fc, ok := r.index[assetKey{typ, file}]
	if !ok {
		return nil, os.ErrNotExist
	}

	// ReadContent reuses one decompressed-frame buffer, so reads are serialised.
	r.mu.Lock()
	defer r.mu.Unlock()
	b, err := r.pkg.ReadContent(&fc)
	if err != nil {
		return nil, err
	}
	// The package keeps reusing its frame buffer; hand back our own copy.
	out := make([]byte, len(b))
	copy(out, b)
	return out, nil
}

// HasPackageAsset reports whether the install contains an asset.
func HasPackageAsset(dataDir, typeHex, fileHex string) bool {
	r, err := openPackageReader(dataDir)
	if err != nil {
		return false
	}
	_, ok := r.index[assetKey{HexToSymbol(typeHex), HexToSymbol(fileHex)}]
	return ok
}
