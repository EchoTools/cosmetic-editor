package data

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeZip builds a zip holding the given name -> content entries.
func writeZip(t *testing.T, files map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "mod.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return p
}

// staging returns an empty staging folder inside a settings-like folder.
func staging(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "input-quest")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func readStaged(t *testing.T, dir, typ, asset string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, typ, asset))
	if err != nil {
		t.Fatalf("%s/%s not staged: %v", typ, asset, err)
	}
	return string(b)
}

func TestInstallModZipLayouts(t *testing.T) {
	tex := PlatformQuest.TextureMetaTypeHash()
	gpu := PlatformQuest.TextureGPUTypeHash()
	cases := map[string]string{
		"flat":          "",
		"wrapped":       "My Cool Mod/",
		"staging-named": "input-quest/",
		"chunk folder":  "0/",
		"both":          "My Mod/input-quest/0/",
	}
	for name, prefix := range cases {
		t.Run(name, func(t *testing.T) {
			dir := staging(t)
			z := writeZip(t, map[string]string{
				prefix + tex + "/2dfe2e7610506f03": "header",
				prefix + gpu + "/2DFE2E7610506F03": "pixels", // upper case is normalised
				prefix + "readme.txt":              "hi",
				prefix + "preview.png":             "png",
			})
			res, err := InstallModZip(z, dir, PlatformQuest)
			if err != nil {
				t.Fatal(err)
			}
			if res.Installed != 2 || res.Types != 2 || res.Ignored != 2 || res.Replaced != 0 {
				t.Errorf("result = %+v", res)
			}
			if got := readStaged(t, dir, tex, "2dfe2e7610506f03"); got != "header" {
				t.Errorf("header = %q", got)
			}
			if got := readStaged(t, dir, gpu, "2dfe2e7610506f03"); got != "pixels" {
				t.Errorf("gpu = %q", got)
			}
			if len(res.Textures) != 1 || res.Textures[0] != "2dfe2e7610506f03" {
				t.Errorf("textures = %v", res.Textures)
			}
		})
	}
}

func TestInstallModZipReplacesStagedFiles(t *testing.T) {
	dir := staging(t)
	tex := PlatformQuest.TextureMetaTypeHash()
	os.MkdirAll(filepath.Join(dir, tex), 0755)
	os.WriteFile(filepath.Join(dir, tex, "abc"), []byte("old"), 0644)

	res, err := InstallModZip(writeZip(t, map[string]string{tex + "/abc": "new"}), dir, PlatformQuest)
	if err != nil {
		t.Fatal(err)
	}
	if res.Replaced != 1 {
		t.Errorf("replaced = %d, want 1", res.Replaced)
	}
	if got := readStaged(t, dir, tex, "abc"); got != "new" {
		t.Errorf("staged = %q, want new", got)
	}
	// Nothing but the staged files themselves is left in the staging folder:
	// the repack scanner would pack a leftover temporary file.
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.Contains(p, ".part") {
			t.Errorf("temporary file left in staging: %s", p)
		}
		return nil
	})
}

func TestInstallModZipRefusesOtherPlatform(t *testing.T) {
	dir := staging(t)
	z := writeZip(t, map[string]string{
		PlatformQuest.TextureMetaTypeHash() + "/abc": "fine",
		PlatformPC.TextureMetaTypeHash() + "/abc":    "pc texture",
	})
	_, err := InstallModZip(z, dir, PlatformQuest)
	if err == nil || !strings.Contains(err.Error(), "PC") {
		t.Fatalf("a PC mod installed into Quest staging: err = %v", err)
	}
	// And nothing was written, not even the valid half.
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("staging folder changed by a refused mod: %v", entries)
	}
}

func TestInstallModZipSkipsCosmeticDB(t *testing.T) {
	dir := staging(t)
	db := PlatformQuest.CosmeticDBTypeHash()
	tex := PlatformQuest.TextureMetaTypeHash()

	res, err := InstallModZip(writeZip(t, map[string]string{
		db + "/" + PlatformQuest.CosmeticDBAssetHashes()[0]: "their database",
		tex + "/abc": "texture",
	}), dir, PlatformQuest)
	if err != nil {
		t.Fatal(err)
	}
	if !res.SkippedDB || res.Installed != 1 {
		t.Errorf("result = %+v, want the database skipped and the texture installed", res)
	}
	if _, err := os.Stat(filepath.Join(dir, db)); err == nil {
		t.Error("the mod's cosmetic database was staged")
	}

	// A mod that is only a database installs nothing, and says why.
	_, err = InstallModZip(writeZip(t, map[string]string{
		db + "/" + PlatformQuest.CosmeticDBAssetHashes()[1]: "db",
	}), dir, PlatformQuest)
	if err == nil || !strings.Contains(err.Error(), "database") {
		t.Errorf("database-only mod: err = %v", err)
	}
}

func TestInstallModZipRejectsNonMods(t *testing.T) {
	dir := staging(t)
	if _, err := InstallModZip(writeZip(t, map[string]string{"readme.txt": "x", "pics/a.png": "y"}), dir, PlatformQuest); err == nil {
		t.Error("a zip with no mod files was accepted")
	}
	notZip := filepath.Join(t.TempDir(), "x.zip")
	os.WriteFile(notZip, []byte("not a zip"), 0644)
	if _, err := InstallModZip(notZip, dir, PlatformQuest); err == nil {
		t.Error("a file that is not a zip was accepted")
	}
	// Path tricks cannot place files outside the staging folder: only the
	// last two names are used, and they must be hashes.
	res, err := InstallModZip(writeZip(t, map[string]string{
		"../../" + PlatformQuest.TextureMetaTypeHash() + "/abc": "x",
	}), dir, PlatformQuest)
	if err != nil || res.Installed != 1 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	readStaged(t, dir, PlatformQuest.TextureMetaTypeHash(), "abc")
}
