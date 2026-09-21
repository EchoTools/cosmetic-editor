package data

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Platform identifies which build of Echo VR a set of assets belongs to.
//
// Echo VR hashes every resource type name with the same rad_hash used for
// CSymbol64 (see ToSymbol), and the type names carry a platform suffix:
// "CGTextureResourceWin10" on PC, "CGTextureResourceAndroid" on Quest.  The
// two therefore land in completely different on-disk folders even though the
// payload grammar is the same.  Nothing here is hardcoded: every hash below is
// derived from its authored name at init time, and platform_test.go pins the
// results against the values verified from the game binaries.
type Platform int

const (
	PlatformPC Platform = iota
	PlatformQuest
)

// TypeSuffix is the platform tag appended to every resource type name.
func (p Platform) TypeSuffix() string {
	if p == PlatformQuest {
		return "Android"
	}
	return "Win10"
}

// String returns the display name used in settings and the UI.
func (p Platform) String() string {
	if p == PlatformQuest {
		return "Quest"
	}
	return "PC"
}

// ParsePlatform maps a persisted settings string to a Platform, defaulting to Quest.
func ParsePlatform(s string) Platform {
	if strings.EqualFold(s, "PC") || strings.EqualFold(s, "PCVR") {
		return PlatformPC
	}
	return PlatformQuest
}

// TypeHash returns the folder hash for a platform-suffixed resource type.
// baseName is the type name without its platform tag, e.g. "CGTextureResource".
func (p Platform) TypeHash(baseName string) string {
	return SymbolToHex(int64(ToSymbol(baseName + p.TypeSuffix())))
}

// GPUTypeHash returns the folder hash for a type's GPU sidecar.
func (p Platform) GPUTypeHash(baseName string) string {
	return SymbolToHex(int64(ToSymbol(baseName + p.TypeSuffix() + "GPU")))
}

// CosmeticDBAssetNames lists the authored asset names the cosmetic database is
// published under, in the order they should be written.
//
// PC ships one: "r14_glb_global_root".  The Quest build ships that name *and*
// "r14_glb_global_root_lowspec", and in a real Quest extract the two files are
// byte-identical to each other and to PC's.  Which one the runtime loads
// depends on the device's spec tier, so an edit written to only one of them
// takes effect on some headsets and silently does nothing on others.  Both are
// therefore written.
func (p Platform) CosmeticDBAssetNames() []string {
	if p == PlatformQuest {
		return []string{"r14_glb_global_root", "r14_glb_global_root_lowspec"}
	}
	return []string{"r14_glb_global_root"}
}

// CosmeticDBAssetHashes maps CosmeticDBAssetNames to on-disk filenames.
func (p Platform) CosmeticDBAssetHashes() []string {
	names := p.CosmeticDBAssetNames()
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = SymbolToHex(int64(ToSymbol(n)))
	}
	return out
}

// CosmeticDBAssetName is the primary asset name for this platform: the one the
// editor reads from when loading.
func (p Platform) CosmeticDBAssetName() string { return p.CosmeticDBAssetNames()[0] }

// CosmeticDBAssetHash is CosmeticDBAssetName hashed to its on-disk filename.
func (p Platform) CosmeticDBAssetHash() string {
	return SymbolToHex(int64(ToSymbol(p.CosmeticDBAssetName())))
}

// CosmeticDBTypeHash is the folder the cosmetic database lives in.
func (p Platform) CosmeticDBTypeHash() string { return p.TypeHash("CR15NetRewardItemCR") }

// TextureMetaTypeHash is the folder holding 256-byte texture descriptors.
func (p Platform) TextureMetaTypeHash() string { return p.TypeHash("CGTextureResource") }

// TextureGPUTypeHash is the folder holding texture pixel payloads.
func (p Platform) TextureGPUTypeHash() string { return p.GPUTypeHash("CGTextureResource") }

// RuntimeDirName is the leaf of the game's _data path that names the build.
func (p Platform) RuntimeDirName() string {
	if p == PlatformQuest {
		return "android"
	}
	return "win10"
}

// InputDirName is the staging folder modified assets are written to before a repack.
func (p Platform) InputDirName() string {
	if p == PlatformQuest {
		return "input-quest"
	}
	return "input-pcvr"
}

// ExtractedDirName is the folder extracted originals are cached in.
func (p Platform) ExtractedDirName() string {
	if p == PlatformQuest {
		return "quest-extracted"
	}
	return "pcvr-extracted"
}

// TextureCodec reports how texture payloads are compressed on this platform.
// PC ships DDS-wrapped BCn; Quest ships raw ASTC blocks behind the same
// 256-byte Echo header.
func (p Platform) TextureCodec() string {
	if p == PlatformQuest {
		return "ASTC"
	}
	return "BCn"
}

// PackageName is the package the cosmetic database ships in.  Both platforms
// use the same package hash; only the runtime directory differs.
const PackageName = "48037dc70b0ecab2"

// QuestDataPath is where Echo VR keeps its data on a Quest.  This is the path
// the texture editor pushes to and reads the manifest from.  It lives under
// Android/media rather than Android/data, which is what makes it readable and
// writable by another app on Android 11+.
const QuestDataPath = "/sdcard/Android/media/com.readyatdawn.r15/files/_data/5932408047/rad15/android"

// questDataRoots are the _data folders scanned when QuestDataPath is absent,
// covering the same tree reached through the other mount point.
var questDataRoots = []string{
	"/sdcard/Android/media/com.readyatdawn.r15/files/_data",
	"/storage/emulated/0/Android/media/com.readyatdawn.r15/files/_data",
}

// FindQuestDataPath returns the Echo VR data directory on this device, or ""
// when the game's files are not present.  The known path is tried first; the
// scan below it only matters if an install uses a different id folder.  A
// directory counts only when it holds the package manifest, so a stale folder
// left by an uninstall is not mistaken for an install.
func FindQuestDataPath() string {
	hasManifest := func(p string) bool {
		_, err := os.Stat(filepath.Join(p, "manifests", PackageName))
		return err == nil
	}
	if hasManifest(QuestDataPath) {
		return QuestDataPath
	}
	for _, root := range questDataRoots {
		ids, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, id := range ids {
			if !id.IsDir() {
				continue
			}
			if p := filepath.Join(root, id.Name(), "rad15", "android"); hasManifest(p) {
				return p
			}
		}
	}
	return ""
}

// IsAndroid reports whether this binary is running on an Android device.
func IsAndroid() bool { return runtime.GOOS == "android" }

// DefaultDataPath returns the game data directory to use before the user has
// picked one: the on-device tree when running on the headset, and the usual
// Oculus install locations when running on a PC.
func DefaultDataPath(p Platform) string {
	if IsAndroid() {
		return FindQuestDataPath()
	}
	if p == PlatformQuest {
		return ""
	}
	return GetDefaultEchoVRPath()
}

// PackageChunkPath returns the path of package chunk n for this data directory.
func PackageChunkPath(dataDir string, n int) string {
	return filepath.Join(dataDir, "packages", fmt.Sprintf("%s_%d", PackageName, n))
}

// CountPackageChunks reports how many package chunks the install currently has.
func CountPackageChunks(dataDir string) int {
	n := 0
	for {
		if _, err := os.Stat(PackageChunkPath(dataDir, n)); err != nil {
			return n
		}
		n++
	}
}

// StockPackageChunks is how many package chunks a clean install ships with.
// A PC build ships three (_0.._2), so the first repack appends _3; a Quest
// build ships one (_0), so it appends _1.  This is only a fallback for an
// install whose backup predates the chunk-count record: the count written
// beside the manifest backup is authoritative, because an install that has
// already been repacked has more chunks than it shipped with.
func (p Platform) StockPackageChunks() int {
	if p == PlatformQuest {
		return 1
	}
	return 3
}

// chunkCountSuffix names the file recording how many chunks existed when the
// manifest backup was taken.
const chunkCountSuffix = ".chunks"

// RecordStockChunkCount notes how many package chunks exist alongside a freshly
// taken manifest backup, so a later revert knows which chunks a repack added.
// The manifest itself cannot answer this: it is a zstd container, so the count
// is not readable at a fixed offset, and parsing it to revert would make revert
// depend on the manifest version.
func RecordStockChunkCount(manifestBackupPath string, count int) error {
	return os.WriteFile(manifestBackupPath+chunkCountSuffix,
		[]byte(fmt.Sprintf("%d\n", count)), 0644)
}

// StockChunkCount returns the chunk count recorded beside a manifest backup,
// falling back to what the platform ships with when no record exists.
func StockChunkCount(manifestBackupPath string, p Platform) int {
	b, err := os.ReadFile(manifestBackupPath + chunkCountSuffix)
	if err != nil {
		return p.StockPackageChunks()
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || n <= 0 {
		return p.StockPackageChunks()
	}
	return n
}

// HasManifest reports whether dir is a game data directory, meaning it holds
// the package manifest the editor repacks into.
func HasManifest(dir string) bool {
	if dir == "" {
		return false
	}
	_, err := os.Stat(ManifestPath(dir))
	return err == nil
}

// ResolveDataPath turns a folder the user picked into the data directory
// beneath it.  People pick the game folder, the _data folder or the runtime
// folder itself, so each level of _data/5932408047/rad15/<runtime> is tried.
// It returns "" when none of them holds a manifest.
func ResolveDataPath(picked string, p Platform) string {
	if picked == "" {
		return ""
	}
	tail := []string{"_data", "5932408047", "rad15", p.RuntimeDirName()}
	candidates := []string{picked}
	for i := range tail {
		candidates = append(candidates, filepath.Join(append([]string{picked}, tail[i:]...)...))
	}
	// A PC install is picked at its ready-at-dawn-echo-arena folder.
	if idx := strings.Index(strings.ToLower(picked), "ready-at-dawn-echo-arena"); idx != -1 {
		base := picked[:idx+len("ready-at-dawn-echo-arena")]
		candidates = append(candidates, filepath.Join(append([]string{base}, tail...)...))
	}
	for _, c := range candidates {
		if HasManifest(c) {
			return c
		}
	}
	return ""
}
