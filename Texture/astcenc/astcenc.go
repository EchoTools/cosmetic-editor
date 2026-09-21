// Package astcenc compresses images to ASTC using ARM's astc-encoder, built
// into the app through cgo.
//
// The encoder is the reference one the game's own textures were made with, so
// replaced textures match the quality of the originals. It used to be run as
// astcenc-avx2.exe, which cannot work on Android: an app may not execute a
// binary it ships in its own data directory. The library sources (5.7.0,
// Apache-2.0, see LICENSE.txt) are compiled into the app instead, with NEON on
// the Quest and SSE4.1 on desktop.
package astcenc

/*
#cgo CXXFLAGS: -std=c++14 -O2 -DASTCENC_NO_INVARIANCE=1
#cgo arm64 CXXFLAGS: -DASTCENC_NEON=1 -DASTCENC_SVE=0 -DASTCENC_SSE=0 -DASTCENC_AVX=0 -DASTCENC_POPCNT=0 -DASTCENC_F16C=0
#cgo amd64 CXXFLAGS: -DASTCENC_NEON=0 -DASTCENC_SVE=0 -DASTCENC_SSE=41 -DASTCENC_AVX=0 -DASTCENC_POPCNT=1 -DASTCENC_F16C=0 -march=nehalem
#cgo !arm64,!amd64 CXXFLAGS: -DASTCENC_NEON=0 -DASTCENC_SVE=0 -DASTCENC_SSE=0 -DASTCENC_AVX=0 -DASTCENC_POPCNT=0 -DASTCENC_F16C=0
#include <stdlib.h>
#include "evr_astcenc.h"
*/
import "C"

import (
	"fmt"
	"image"
	"image/draw"
	"runtime"
	"sync"
	"unsafe"
)

// Quality presets, matching astcenc's own.
const (
	QualityFastest    = 0
	QualityFast       = 10
	QualityMedium     = 60
	QualityThorough   = 98
	QualityExhaustive = 100
)

// Encode compresses img into raw ASTC blocks (no .astc file header), one block
// of blockW x blockH texels per 16 bytes, in row-major block order. srgb picks
// the sRGB profile, which is what the game's *_SRGB formats expect.
func Encode(img image.Image, blockW, blockH int, srgb bool, quality float32) ([]byte, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("empty image")
	}
	if blockW <= 0 || blockH <= 0 {
		return nil, fmt.Errorf("invalid block size %dx%d", blockW, blockH)
	}

	// astcenc wants tightly packed, non-premultiplied RGBA8.
	rgba := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)

	// The pixels are handed to C for the whole compression, so they live in C
	// memory rather than on the Go heap, where they could not be retained.
	pix := C.CBytes(rgba.Pix)
	defer C.free(pix)

	blocks := ((w + blockW - 1) / blockW) * ((h + blockH - 1) / blockH)
	outLen := blocks * 16
	out := C.malloc(C.size_t(outLen))
	defer C.free(out)

	threads := runtime.NumCPU()
	if threads > 8 {
		threads = 8
	}
	if threads < 1 {
		threads = 1
	}

	errBuf := (*C.char)(C.malloc(256))
	defer C.free(unsafe.Pointer(errBuf))
	srgbFlag := C.int(0)
	if srgb {
		srgbFlag = 1
	}
	job := C.evr_astc_begin((*C.uint8_t)(pix), C.unsigned(w), C.unsigned(h),
		C.unsigned(blockW), C.unsigned(blockH), srgbFlag, C.float(quality),
		C.unsigned(threads), errBuf, 256)
	if job == nil {
		return nil, fmt.Errorf("astcenc: %s", C.GoString(errBuf))
	}
	defer C.evr_astc_end(job)

	// astcenc shares one image's work between however many threads call it,
	// each with its own index.
	var wg sync.WaitGroup
	errs := make([]error, threads)
	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			eb := (*C.char)(C.malloc(256))
			defer C.free(unsafe.Pointer(eb))
			if C.evr_astc_run(job, (*C.uint8_t)(out), C.size_t(outLen), C.unsigned(i), eb, 256) != 0 {
				errs[i] = fmt.Errorf("astcenc: %s", C.GoString(eb))
			}
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return C.GoBytes(out, C.int(outLen)), nil
}
