// Thin C interface over the astcenc library, so Go can drive it through cgo
// without seeing any C++.
#ifndef EVR_ASTCENC_H
#define EVR_ASTCENC_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct evr_astc_job evr_astc_job;

// evr_astc_begin prepares to compress one RGBA8 surface. rgba must stay valid
// until evr_astc_end. Returns NULL and fills err on failure.
evr_astc_job* evr_astc_begin(const uint8_t* rgba, unsigned width, unsigned height,
                             unsigned block_x, unsigned block_y, int srgb,
                             float quality, unsigned threads,
                             char* err, size_t err_len);

// evr_astc_run compresses the surface into out. Call it once from each of
// `threads` threads with a distinct thread_index; the work is shared between
// them. Returns 0 on success.
int evr_astc_run(evr_astc_job* job, uint8_t* out, size_t out_len,
                 unsigned thread_index, char* err, size_t err_len);

// evr_astc_end releases the job.
void evr_astc_end(evr_astc_job* job);

#ifdef __cplusplus
}
#endif

#endif
