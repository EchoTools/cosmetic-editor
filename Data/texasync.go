package data

import (
	"os"
	"path/filepath"
	"sync"

	"fyne.io/fyne/v2"
)

// Texture previews are decoded off the UI thread.
//
// Decoding used to happen inline, so selecting a banner froze the app until its
// texture had been read from the package, decoded and written out as a PNG, and
// the first texture of a session also paid for parsing the package index. Now
// a request returns straight away: a preview that is already cached is used at
// once, and one that is not is decoded in the background and handed back on
// the UI thread when it is ready.

var (
	inflightMu sync.Mutex
	inflight   = map[string][]func(string){}
)

// cachedPreviewPath returns the preview PNG for a texture if it already exists.
func cachedPreviewPath(state *AppState, safe string) (string, bool) {
	if state.Settings.TextureCachePath == "" {
		return "", false
	}
	p := filepath.Join(state.Settings.TextureCachePath, safe+".png")
	if _, err := os.Stat(p); err != nil {
		return "", false
	}
	return p, true
}

// RequestTexture makes sure a texture's preview exists and calls onReady on the
// UI thread with its path, or with "" if it could not be decoded. A preview
// that is already cached is delivered immediately. Concurrent requests for the
// same texture share one decode.
//
// RequestTexture must be called on the UI thread.
func RequestTexture(state *AppState, hexStr string, onReady func(path string)) {
	safe, err := SafeHexFilename(hexStr)
	if err != nil || safe == "ffffffffffffffff" {
		onReady("")
		return
	}
	if p, ok := cachedPreviewPath(state, safe); ok {
		onReady(p)
		return
	}

	inflightMu.Lock()
	waiting, busy := inflight[safe]
	inflight[safe] = append(waiting, onReady)
	inflightMu.Unlock()
	if busy {
		return
	}

	go func() {
		err := CacheTexturePNG(state, safe)
		path := ""
		if err == nil {
			path, _ = cachedPreviewPath(state, safe)
		}

		inflightMu.Lock()
		callbacks := inflight[safe]
		delete(inflight, safe)
		inflightMu.Unlock()

		fyne.Do(func() {
			if err != nil && state.StatusLabel != nil {
				state.StatusLabel.SetText("Texture " + safe + ": " + err.Error())
			}
			for _, cb := range callbacks {
				cb(path)
			}
		})
	}()
}

// CacheTextureQuietly makes sure a texture's preview exists, without touching
// the UI. It is for background goroutines, such as the emote player filling in
// its frames.
func CacheTextureQuietly(state *AppState, hexStr string) {
	safe, err := SafeHexFilename(hexStr)
	if err != nil {
		return
	}
	if _, ok := cachedPreviewPath(state, safe); ok {
		return
	}
	CacheTexturePNG(state, safe)
}

// WarmPackageReader opens the install's package index in the background, so the
// first preview of a session does not pay for parsing it.
func WarmPackageReader(dataDir string) {
	if !HasManifest(dataDir) {
		return
	}
	go openPackageReader(dataDir)
}
