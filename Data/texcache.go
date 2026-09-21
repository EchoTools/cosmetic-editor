package data

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	texture "github.com/EchoTools/cosmetic-editor/Texture"
)

// DecodeTextureAsset loads a texture by its asset hash and decodes it to an
// image, entirely in process.
//
// A texture is two files under two different type folders: a header in the
// CGTextureResource folder and, when the texels are not inlined in that header,
// the payload in the matching GPU folder. Which platform's folders to look in
// decides how the header's format field is read, so the platform is passed
// explicitly rather than guessed from the bytes.
func DecodeTextureAsset(p Platform, roots []string, hexStr string) (image.Image, error) {
	sym := HexToSymbol(hexStr)
	if sym == -1 {
		return nil, fmt.Errorf("invalid asset hash %q", hexStr)
	}

	metaFolder := p.TextureMetaTypeHash()
	gpuFolder := p.TextureGPUTypeHash()

	var headerPath, payloadPath string
	for _, root := range roots {
		if root == "" {
			continue
		}
		if headerPath == "" {
			headerPath = FindExtractedAsset(root, sym, metaFolder)
		}
		if payloadPath == "" {
			payloadPath = FindExtractedAsset(root, sym, gpuFolder)
		}
		if headerPath != "" && payloadPath != "" {
			break
		}
	}
	if headerPath == "" {
		return nil, fmt.Errorf("no texture header for %s", hexStr)
	}

	headerBytes, err := os.ReadFile(headerPath)
	if err != nil {
		return nil, err
	}
	desc, err := texture.ParseDescriptor(headerBytes, p == PlatformQuest)
	if err != nil {
		return nil, err
	}

	payload := desc.Payload
	if len(payload) == 0 {
		if payloadPath == "" {
			return nil, fmt.Errorf("texture %s has no inline texels and no GPU payload", hexStr)
		}
		if payload, err = os.ReadFile(payloadPath); err != nil {
			return nil, err
		}
	}
	return texture.Decode(desc, payload)
}

// textureSearchRoots lists the folders a texture may be found in, newest first,
// so a freshly staged replacement is preferred over the extracted original.
func textureSearchRoots(state *AppState) []string {
	p := state.Platform()
	extracted := state.Settings.ExtractedPath
	if extracted == "" {
		extracted = filepath.Join(GetSettingsDir(), p.ExtractedDirName())
	}
	return []string{state.StagingDir(), extracted}
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

	img, err := DecodeTextureAsset(state.Platform(), textureSearchRoots(state), safe)
	if err != nil {
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
