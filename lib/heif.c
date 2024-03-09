#include <stdlib.h>
#include <string.h>

#include "libheif/heif.h"

void *allocate(size_t size);
void deallocate(void *ptr);

int decode(uint8_t *heic_in, int heic_in_size, int config_only, uint32_t *width, uint32_t *height, uint32_t *premultiplied, uint8_t *rgb_out);

__attribute__((export_name("allocate")))
void *allocate(size_t size) {
    return malloc(size);
}

__attribute__((export_name("deallocate")))
void deallocate(void *ptr) {
    free(ptr);
}

__attribute__((export_name("decode")))
int decode(uint8_t *heic_in, int heic_in_size, int config_only, uint32_t *width, uint32_t *height, uint32_t *stride, uint8_t *rgb_out) {
    enum heif_filetype_result filetype_check = heif_check_filetype(heic_in, heic_in_size);
    if(filetype_check != heif_filetype_yes_supported) {
        return 0;
    }

    struct heif_error err;

    struct heif_context *context = heif_context_alloc();
    err = heif_context_read_from_memory_without_copy(context, heic_in, heic_in_size, NULL);
    if(err.code != heif_error_Ok) {
        heif_context_free(context);
        return 0;
    }

    heif_context_set_max_decoding_threads(context, 0);

    struct heif_image_handle *handle;
    err = heif_context_get_primary_image_handle(context, &handle);
    if(err.code != heif_error_Ok) {
        heif_context_free(context);
        return 0;
    }

    *width = (uint32_t)heif_image_handle_get_width(handle);
    *height = (uint32_t)heif_image_handle_get_height(handle);

    struct heif_image *img;
    err = heif_image_create(*width, *height, heif_colorspace_RGB, heif_chroma_interleaved_RGBA, &img);
    if(err.code != heif_error_Ok) {
        heif_context_free(context);
        heif_image_handle_release(handle);
        return 0;
    }

    err = heif_image_add_plane(img, heif_channel_interleaved, *width, *height, 4);
    if(err.code != heif_error_Ok) {
        heif_context_free(context);
        heif_image_handle_release(handle);
        return 0;
    }

    heif_image_get_plane_readonly(img, heif_channel_interleaved, (int *)stride);

    if(config_only) {
        heif_context_free(context);
        heif_image_handle_release(handle);
        return 1;
    }

    struct heif_decoding_options* options = heif_decoding_options_alloc();
    options->convert_hdr_to_8bit = 1;

    err = heif_decode_image(handle, &img, heif_colorspace_RGB, heif_chroma_interleaved_RGBA, options);
    if(err.code != heif_error_Ok) {
        heif_context_free(context);
        heif_image_handle_release(handle);
        return 0;
    }

    const uint8_t *image = heif_image_get_plane_readonly(img, heif_channel_interleaved, NULL);

    int buf_size = *width * *height * 4;
    memcpy(rgb_out, image, buf_size);

    heif_decoding_options_free(options);
    heif_context_free(context);
    heif_image_handle_release(handle);
    heif_image_release(img);
    return 1;
}

void __cxa_allocate_exception() {
    abort();
}

void __cxa_throw() {
    abort();
}
