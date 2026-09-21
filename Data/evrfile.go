package data

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/EchoTools/cosmetic-editor/EvrFile/manifest"
)

// This file runs Echo VR's package format in process.  The editor used to shell
// out to evrFileTools.exe for both extract and repack, which cannot work on
// Android: an app may not execute a binary it ships in its own data directory.
// The package code is pure Go, so it is vendored under EvrFile and called
// directly.

// ExtractKind names a group of resource types worth pulling out of a package.
type ExtractKind string

const (
	ExtractTints    ExtractKind = "tints"
	ExtractTextures ExtractKind = "textures"
)

// typeHashes returns the resource type folder hashes for a kind.  Both
// platforms' hashes are included so an extract of either build works without
// the caller having to say which it is; the folder names keep them apart.
func (k ExtractKind) typeHashes() []int64 {
	var names []string
	switch k {
	case ExtractTints:
		names = []string{
			PlatformPC.CosmeticDBTypeHash(),
			PlatformQuest.CosmeticDBTypeHash(),
		}
	case ExtractTextures:
		names = []string{
			PlatformPC.TextureMetaTypeHash(), PlatformPC.TextureGPUTypeHash(),
			PlatformQuest.TextureMetaTypeHash(), PlatformQuest.TextureGPUTypeHash(),
		}
	}
	out := make([]int64, 0, len(names))
	for _, n := range names {
		out = append(out, HexToSymbol(n))
	}
	return out
}

// ManifestPath is the path of a data directory's package manifest.
func ManifestPath(dataDir string) string {
	return filepath.Join(dataDir, "manifests", PackageName)
}

// ExtractPackage pulls the requested resource kinds out of the install at
// dataDir and writes them under outputDir, one folder per type hash.
func ExtractPackage(dataDir, outputDir string, kinds ...ExtractKind) error {
	m, err := manifest.ReadFile(ManifestPath(dataDir))
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	pkg, err := manifest.OpenPackage(m, filepath.Join(dataDir, "packages", PackageName))
	if err != nil {
		return fmt.Errorf("open package: %w", err)
	}
	defer pkg.Close()

	var types []int64
	for _, k := range kinds {
		types = append(types, k.typeHashes()...)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	// Extract flat, as <output>/<type hash>/<asset hash>.  Preserving the
	// package's own grouping would insert a numeric folder above the type, and
	// FindExtractedAsset — and every existing extract on disk — expects the
	// type folder directly under the root.
	opts := []manifest.ExtractOption{manifest.WithPreserveGroups(false)}
	if len(types) > 0 {
		opts = append(opts, manifest.WithTypeFilter(types))
	}
	if err := pkg.Extract(outputDir, opts...); err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	return nil
}

// RepackInto merges the staged files under inputDir back into the install at
// dataDir, appending a package chunk and rewriting the manifest in place.
//
// The manifest backup and the stock chunk count must already have been taken:
// once this runs the install is no longer in its shipped state, so recording
// the original chunk count afterwards would record the wrong number.
func RepackInto(dataDir, inputDir string) error {
	files, err := manifest.ScanFiles(inputDir)
	if err != nil {
		return fmt.Errorf("scan staged files: %w", err)
	}
	staged := 0
	for _, g := range files {
		staged += len(g)
	}
	if staged == 0 {
		return fmt.Errorf("no staged files found in %s", inputDir)
	}

	m, err := manifest.ReadFile(ManifestPath(dataDir))
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	if err := manifest.QuickRepack(m, files, dataDir, PackageName); err != nil {
		return fmt.Errorf("repack: %w", err)
	}
	return nil
}
