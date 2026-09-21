package data

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

// CosmeticDBAssetName is the authored name of the asset holding the cosmetic
// database.  This is the one place where the two platforms disagree on the
// asset itself rather than merely on its type: PC ships "r14_glb_global_root"
// and Quest ships the reduced "r14_glb_global_root_lowspec".  Writing the PC
// name into a Quest build produces a file the game never loads.
func (p Platform) CosmeticDBAssetName() string {
	if p == PlatformQuest {
		return "r14_glb_global_root_lowspec"
	}
	return "r14_glb_global_root"
}

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

// QuestDataPath is where Echo VR keeps its _data tree on a Quest.  This lives
// under Android/media rather than Android/data, which is what makes it
// readable and writable by another app on Android 11+ without the package
// having to grant anything.
const QuestDataPath = "/sdcard/Android/media/com.readyatdawn.r15/files/_data/5932408047/rad15/android"

// questDataPathAlternates are other locations the same tree shows up in,
// depending on how the headset exposes its primary volume.
var questDataPathAlternates = []string{
	"/storage/emulated/0/Android/media/com.readyatdawn.r15/files/_data/5932408047/rad15/android",
	"/sdcard/Android/data/com.readyatdawn.r15/files/_data/5932408047/rad15/android",
}

// FindQuestDataPath returns the first Echo VR _data directory that exists on
// this device, or "" when the game's files are not present.  A directory only
// counts when it actually holds the package manifest, so a stale empty folder
// left behind by an uninstall is not mistaken for an install.
func FindQuestDataPath() string {
	for _, p := range append([]string{QuestDataPath}, questDataPathAlternates...) {
		if _, err := os.Stat(filepath.Join(p, "manifests", PackageName)); err == nil {
			return p
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
// PC builds ship three (_0.._2) and Quest ships one (_0), which is why a repack
// appends a different chunk number on each platform; assuming PC's _3 on a
// Quest leaves an orphaned file the manifest never points at.
func CountPackageChunks(dataDir string) int {
	n := 0
	for {
		if _, err := os.Stat(PackageChunkPath(dataDir, n)); err != nil {
			return n
		}
		n++
	}
}
