package data

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"

	texture "github.com/EchoTools/cosmetic-editor/Texture"
)

// textureSource finds one file of a texture: its header (the type folder) or
// its GPU payload.  It returns os.ErrNotExist when the file is not present.
type textureSource func(typeHex, fileHex string) ([]byte, error)

// diskSource looks for files under folders laid out as <root>/<type>/<asset>,
// which is how staged replacements (and any existing extract) are stored.
func diskSource(roots ...string) textureSource {
	return func(typeHex, fileHex string) ([]byte, error) {
		sym := HexToSymbol(fileHex)
		for _, root := range roots {
			if root == "" {
				continue
			}
			if p := FindExtractedAsset(root, sym, typeHex); p != "" {
				return os.ReadFile(p)
			}
		}
		return nil, os.ErrNotExist
	}
}

// packageSource reads files straight out of the game's package.
func packageSource(dataDir string) textureSource {
	return func(typeHex, fileHex string) ([]byte, error) {
		return ReadPackageAsset(dataDir, typeHex, fileHex)
	}
}

// firstOf tries each source in turn.  Header and payload are looked up
// separately on purpose: a staged replacement may carry only one of them.
func firstOf(sources ...textureSource) textureSource {
	return func(typeHex, fileHex string) ([]byte, error) {
		for _, src := range sources {
			b, err := src(typeHex, fileHex)
			if err == nil {
				return b, nil
			}
			if !os.IsNotExist(err) {
				return nil, err
			}
		}
		return nil, os.ErrNotExist
	}
}

// DecodeTextureAsset loads a texture by its asset hash and decodes it to an
// image, entirely in process.
//
// A texture is two files under two different type folders: a header in the
// CGTextureResource folder and, when the texels are not inlined in that header,
// the payload in the matching GPU folder.  Which platform's folders to look in
// decides how the header's format field is read, so the platform is passed
// explicitly rather than guessed from the bytes.
func DecodeTextureAsset(p Platform, src textureSource, hexStr string) (image.Image, error) {
	desc, err := LoadTextureHeader(p, src, hexStr)
	if err != nil {
		return nil, err
	}
	payload := desc.Payload
	if len(payload) == 0 {
		payload, err = src(p.TextureGPUTypeHash(), hexStr)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("texture %s has no inline texels and no GPU payload", hexStr)
			}
			return nil, err
		}
	}
	return texture.Decode(desc, payload)
}

// LoadTextureHeader reads and parses a texture's header.
func LoadTextureHeader(p Platform, src textureSource, hexStr string) (*texture.Descriptor, error) {
	if HexToSymbol(hexStr) == -1 {
		return nil, fmt.Errorf("invalid asset hash %q", hexStr)
	}
	b, err := src(p.TextureMetaTypeHash(), hexStr)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no texture header for %s", hexStr)
		}
		return nil, err
	}
	return texture.ParseDescriptor(b, p == PlatformQuest)
}

// textureSourceFor is where the app looks for a texture: staged replacements
// first, so a texture that has just been replaced previews as the replacement,
// then an existing extract if there is one, then the game's own package.
func textureSourceFor(state *AppState) textureSource {
	p := state.Platform()
	extracted := state.Settings.ExtractedPath
	if extracted == "" {
		extracted = filepath.Join(GetSettingsDir(), p.ExtractedDirName())
	}
	return firstOf(
		diskSource(state.StagingDir(), extracted),
		packageSource(state.Settings.EchoVRDataPath),
	)
}

// CacheTexturePNG decodes a texture asset and writes it to the preview cache.
func CacheTexturePNG(state *AppState, hexStr string) error {
	safe, err := SafeHexFilename(hexStr)
	if err != nil {
		return err
	}

	cacheDir := state.Settings.TextureCachePath
	if cacheDir == "" {
		cacheDir = filepath.Join(GetSettingsDir(), "texture_cache")
		state.Settings.TextureCachePath = cacheDir
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	img, err := DecodeTextureAsset(state.Platform(), textureSourceFor(state), safe)
	if err != nil {
		// PC textures are BC-compressed, which the in-process decoder does not
		// handle.  The Windows build still has texconv for those; a headset
		// only ever sees Quest textures, which are ASTC.
		if !IsAndroid() {
			if ferr := cacheViaTexconv(state, safe, cacheDir); ferr == nil {
				return nil
			}
		}
		return err
	}

	// Write via a temporary file so a cancelled or failed write cannot leave a
	// truncated PNG that later loads as a broken preview.
	tmp, err := os.CreateTemp(cacheDir, safe+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if err := png.Encode(tmp, img); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, filepath.Join(cacheDir, safe+".png"))
}

// cacheViaTexconv writes a PC texture's DDS payload to a temporary file and
// has texconv convert it, for the BC formats the in-process decoder lacks.
func cacheViaTexconv(state *AppState, hexStr, cacheDir string) error {
	p := state.Platform()
	src := textureSourceFor(state)
	desc, err := LoadTextureHeader(p, src, hexStr)
	if err != nil {
		return err
	}
	payload := desc.Payload
	if len(payload) == 0 {
		if payload, err = src(p.TextureGPUTypeHash(), hexStr); err != nil {
			return err
		}
	}
	if len(payload) < 4 || string(payload[:4]) != "DDS " {
		return fmt.Errorf("texture %s payload is not a DDS file", hexStr)
	}

	texconv, err := FindTool(GetSettingsDir(), "ms_texconv.exe")
	if err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp("", "evrtex")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	dds := filepath.Join(tmpDir, hexStr+".dds")
	if err := os.WriteFile(dds, payload, 0644); err != nil {
		return err
	}
	cmd := exec.Command(texconv, "-ft", "png", "-o", cacheDir, "-y", dds)
	cmd.SysProcAttr = HiddenProcAttr()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("texconv: %v: %s", err, out)
	}
	return nil
}
