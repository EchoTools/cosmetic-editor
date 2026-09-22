package data

import (
	"fmt"
	"path/filepath"

	"github.com/EchoTools/cosmetic-editor/EvrFile/manifest"
)

// This file runs Echo VR's package format in process.  The editor used to shell
// out to evrFileTools.exe, which cannot work on Android: an app may not execute
// a binary it ships in its own data directory.  The package code is pure Go, so
// it is vendored under EvrFile and called directly.
//
// Only repack lives here.  Nothing is extracted any more: a full extract is
// several gigabytes, which a headset cannot spare, and the editor never needed
// most of it.  The cosmetic database is built in, and single assets are read
// from the package on demand (see pkgread.go).

// ManifestPath is the path of a data directory's package manifest.
func ManifestPath(dataDir string) string {
	return filepath.Join(dataDir, "manifests", PackageName)
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
