package data

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Mods made by other tools are shared as zips of evrFileTools input files:
// one folder per resource type, named by its hash, holding one file per
// resource, also named by its hash. Installing a mod copies those files into
// the staging folder, where the next repack picks them up alongside the
// editor's own changes.
//
// Zips are not laid out consistently. Some have the type folders at the top,
// some wrap them in a folder named after the mod or after the staging folder,
// and some keep evrFileTools' chunk folder ("0/<type>/<file>"). So each entry
// is placed by its last two path components alone, and anything that is not a
// type/file pair of hashes (a readme, a preview image) is left out.

const (
	// maxModFile and maxModTotal stop a malformed or hostile zip from filling
	// the headset's storage. The largest shipped texture is a few megabytes.
	maxModFile  = 256 << 20
	maxModTotal = 2 << 30
)

// ModInstallResult describes what InstallModZip did.
type ModInstallResult struct {
	Installed int // files written to the staging folder
	Replaced  int // of those, how many replaced a file already staged
	Types     int // distinct resource types the mod touches
	Ignored   int // zip entries that are not mod files, such as readmes
	// SkippedDB is set when the mod carried its own cosmetic database. It is
	// left out: the editor writes the database itself on every repack, so the
	// mod's copy would be replaced anyway, and its cosmetic edits belong in
	// the editor instead.
	SkippedDB bool
	// Textures lists the texture resources the mod replaced, so their cached
	// previews can be refreshed.
	Textures []string
}

// otherPlatformTypes are the type folders the other platform's build uses. A
// mod holding one of them was made for the other build: its files would be
// added under type names this build never loads, so it is refused outright.
func otherPlatformTypes(p Platform) map[string]bool {
	other := PlatformPC
	if p == PlatformPC {
		other = PlatformQuest
	}
	out := map[string]bool{}
	for _, base := range []string{
		"CGTextureResource", "CR15NetRewardItemCR", "CGMeshListResource",
		"CGInstancedModelResource", "CGMaterialResource",
		"CGTextureStreamingResource", "CArchiveResource",
	} {
		out[other.TypeHash(base)] = true
		out[other.GPUTypeHash(base)] = true
	}
	return out
}

// symbolName reports whether s is a resource hash as the tools write them:
// up to sixteen hex digits. It returns the canonical lower-case form.
func symbolName(s string) (string, bool) {
	if len(s) == 0 || len(s) > 16 {
		return "", false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return "", false
		}
	}
	return strings.ToLower(s), true
}

// modEntry is a zip entry that is a mod file, with where it goes.
type modEntry struct {
	f          *zip.File
	typ, asset string
}

// InstallModZip copies the mod files in a zip into stagingDir for platform p.
// Nothing is written unless the whole zip checks out.
func InstallModZip(zipPath, stagingDir string, p Platform) (ModInstallResult, error) {
	var res ModInstallResult
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return res, fmt.Errorf("not a readable zip: %w", err)
	}
	defer zr.Close()

	wrong := otherPlatformTypes(p)
	dbType := p.CosmeticDBTypeHash()
	dbAssets := map[string]bool{}
	for _, h := range p.CosmeticDBAssetHashes() {
		dbAssets[h] = true
	}

	// Check everything first, so a bad zip leaves the staging folder alone.
	var entries []modEntry
	var total uint64
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		parts := strings.Split(path.Clean(strings.ReplaceAll(f.Name, "\\", "/")), "/")
		if len(parts) < 2 {
			res.Ignored++
			continue
		}
		typ, ok1 := symbolName(parts[len(parts)-2])
		asset, ok2 := symbolName(parts[len(parts)-1])
		if !ok1 || !ok2 {
			res.Ignored++
			continue
		}
		if wrong[typ] {
			return res, fmt.Errorf("this mod is for %s Echo VR, not %s: it contains %s files",
				map[Platform]string{PlatformPC: "Quest", PlatformQuest: "PC"}[p], p, typ)
		}
		if typ == dbType && dbAssets[asset] {
			res.SkippedDB = true
			continue
		}
		if f.UncompressedSize64 > maxModFile {
			return res, fmt.Errorf("%s is too large (%d MB)", f.Name, f.UncompressedSize64>>20)
		}
		total += f.UncompressedSize64
		if total > maxModTotal {
			return res, errors.New("the mod is too large to install (over 2 GB unpacked)")
		}
		entries = append(entries, modEntry{f, typ, asset})
	}
	if len(entries) == 0 {
		if res.SkippedDB {
			return res, errors.New("this mod only changes the cosmetic database; make those changes in the editor instead")
		}
		return res, errors.New("no mod files found: expected folders named by type hash holding files named by asset hash")
	}

	// Temporary files go beside the staging folder, not in it: the repack
	// scanner ignores extensions, so a leftover "<hash>.part" would be packed.
	tmpDir := filepath.Join(filepath.Dir(stagingDir), "Temp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return res, err
	}

	types := map[string]bool{}
	texType := p.TextureMetaTypeHash()
	texGPU := p.TextureGPUTypeHash()
	textures := map[string]bool{}
	for _, e := range entries {
		dir := filepath.Join(stagingDir, e.typ)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return res, err
		}
		dst := filepath.Join(dir, e.asset)
		existed := false
		if _, err := os.Stat(dst); err == nil {
			existed = true
		}
		if err := extractModFile(e.f, dst, tmpDir); err != nil {
			return res, fmt.Errorf("%s: %w", e.f.Name, err)
		}
		res.Installed++
		if existed {
			res.Replaced++
		}
		types[e.typ] = true
		if e.typ == texType || e.typ == texGPU {
			textures[e.asset] = true
		}
	}
	res.Types = len(types)
	for t := range textures {
		res.Textures = append(res.Textures, t)
	}
	return res, nil
}

// extractModFile writes one zip entry to dst, through a temporary file in
// tmpDir so a failed write never leaves a truncated resource behind.
func extractModFile(f *zip.File, dst, tmpDir string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.CreateTemp(tmpDir, "mod-*.part")
	if err != nil {
		return err
	}
	tmp := out.Name()
	// The size in the zip header can lie; never write more than the limit.
	n, err := io.Copy(out, io.LimitReader(rc, maxModFile+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > maxModFile {
		err = errors.New("file is too large")
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	os.Remove(dst)
	return os.Rename(tmp, dst)
}

// ForgetTexturePreview deletes a texture's cached preview, so the next time it
// is shown it is decoded again from whatever is now staged for it.
func ForgetTexturePreview(state *AppState, hexStr string) {
	safe, err := SafeHexFilename(hexStr)
	if err != nil || state.Settings.TextureCachePath == "" {
		return
	}
	os.Remove(filepath.Join(state.Settings.TextureCachePath, safe+".png"))
}
