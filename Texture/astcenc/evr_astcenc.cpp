// C wrapper around astcenc for the Go binding in astcenc.go.

#include "evr_astcenc.h"

#include <cstdio>
#include <new>

#include "astcenc.h"

struct evr_astc_job {
	astcenc_context* ctx;
	astcenc_image image;
	void* slice;
};

static void set_err(char* err, size_t err_len, const char* what, astcenc_error status) {
	if (err && err_len) {
		std::snprintf(err, err_len, "%s: %s", what, astcenc_get_error_string(status));
	}
}

extern "C" evr_astc_job* evr_astc_begin(const uint8_t* rgba, unsigned width, unsigned height,
                                        unsigned block_x, unsigned block_y, int srgb,
                                        float quality, unsigned threads,
                                        char* err, size_t err_len) {
	astcenc_config config;
	astcenc_profile profile = srgb ? ASTCENC_PRF_LDR_SRGB : ASTCENC_PRF_LDR;
	astcenc_error status = astcenc_config_init(profile, block_x, block_y, 1, quality, 0, &config);
	if (status != ASTCENC_SUCCESS) {
		set_err(err, err_len, "config", status);
		return nullptr;
	}

	evr_astc_job* job = new (std::nothrow) evr_astc_job();
	if (!job) {
		if (err && err_len) std::snprintf(err, err_len, "out of memory");
		return nullptr;
	}

	status = astcenc_context_alloc(&config, threads ? threads : 1, &job->ctx, nullptr);
	if (status != ASTCENC_SUCCESS) {
		set_err(err, err_len, "context", status);
		delete job;
		return nullptr;
	}

	// astcenc only reads the input; the cast drops const for its struct.
	job->slice = const_cast<uint8_t*>(rgba);
	job->image.dim_x = width;
	job->image.dim_y = height;
	job->image.dim_z = 1;
	job->image.data_type = ASTCENC_TYPE_U8;
	job->image.data = &job->slice;
	return job;
}

extern "C" int evr_astc_run(evr_astc_job* job, uint8_t* out, size_t out_len,
                            unsigned thread_index, char* err, size_t err_len) {
	static const astcenc_swizzle identity {
		ASTCENC_SWZ_R, ASTCENC_SWZ_G, ASTCENC_SWZ_B, ASTCENC_SWZ_A
	};
	astcenc_error status = astcenc_compress_image(job->ctx, &job->image, &identity,
	                                              out, out_len, thread_index);
	if (status != ASTCENC_SUCCESS) {
		set_err(err, err_len, "compress", status);
		return 1;
	}
	return 0;
}

extern "C" void evr_astc_end(evr_astc_job* job) {
	if (!job) return;
	astcenc_context_free(job->ctx);
	delete job;
}
