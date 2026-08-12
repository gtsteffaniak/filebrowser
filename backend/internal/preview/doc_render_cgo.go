//go:build mupdf

package preview

/*
#cgo CFLAGS: -I${SRCDIR}/gofitzinclude

#include <mupdf/fitz.h>
#include <stdlib.h>
#include <string.h>

static void fb_silence_warnings(void *user, const char *message) {
	(void)user;
	(void)message;
}

// fb_render_document_page_jpeg renders one page to JPEG bytes.
// Returns 0 on success; on failure writes a message to err_buf and returns -1.
// out is malloc'd; caller must free.
static int fb_render_document_page_jpeg(
	const char *path,
	int page_num,
	int jpeg_quality,
	unsigned char **out,
	size_t *out_len,
	char *err_buf,
	size_t err_buf_len
) {
	fz_context *ctx = NULL;
	fz_document *doc = NULL;
	fz_page *page = NULL;
	fz_pixmap *pixmap = NULL;
	fz_device *device = NULL;
	fz_buffer *buf = NULL;
	unsigned char *jpeg_data = NULL;
	size_t jpeg_len = 0;
	int rc = -1;

	if (!path || !out || !out_len || !err_buf || err_buf_len == 0) {
		return -1;
	}

	*out = NULL;
	*out_len = 0;
	memset(err_buf, 0, err_buf_len);

	ctx = fz_new_context_imp(NULL, NULL, 256 << 20, FZ_VERSION);
	if (!ctx) {
		snprintf(err_buf, err_buf_len, "cannot create mupdf context");
		return -1;
	}

	fz_set_warning_callback(ctx, fb_silence_warnings, NULL);

	fz_var(doc);
	fz_var(page);
	fz_var(pixmap);
	fz_var(device);
	fz_var(buf);
	fz_var(jpeg_data);
	fz_var(jpeg_len);
	fz_var(rc);

	fz_try(ctx) {
		fz_register_document_handlers(ctx);

		doc = fz_open_document(ctx, path);
		if (fz_needs_password(ctx, doc)) {
			fz_throw(ctx, FZ_ERROR_ARGUMENT, "document needs password");
		}

		int num_pages = fz_count_pages(ctx, doc);
		if (page_num < 0 || page_num >= num_pages) {
			fz_throw(ctx, FZ_ERROR_ARGUMENT, "invalid page number");
		}

		page = fz_load_page(ctx, doc, page_num);

		// Match go-fitz ImageDPI scale used before resize in the preview pipeline.
		double dpi = 150.0;
		fz_rect bounds = fz_bound_page(ctx, page);
		fz_matrix ctm = fz_scale(dpi / 72.0, dpi / 72.0);
		bounds = fz_transform_rect(bounds, ctm);
		fz_irect bbox = fz_round_rect(bounds);

		pixmap = fz_new_pixmap_with_bbox(ctx, fz_device_rgb(ctx), bbox, NULL, 0);
		fz_clear_pixmap_with_value(ctx, pixmap, 0xff);

		device = fz_new_draw_device(ctx, ctm, pixmap);
		fz_enable_device_hints(ctx, device, FZ_NO_CACHE);
		fz_run_page_contents(ctx, page, device, fz_identity, NULL);
		fz_close_device(ctx, device);
		fz_drop_device(ctx, device);
		device = NULL;

		buf = fz_new_buffer_from_pixmap_as_jpeg(ctx, pixmap, fz_default_color_params, jpeg_quality, 0);
		jpeg_len = fz_buffer_storage(ctx, buf, &jpeg_data);
		if (jpeg_len == 0 || jpeg_data == NULL) {
			fz_throw(ctx, FZ_ERROR_GENERIC, "empty jpeg buffer");
		}

		*out = (unsigned char *)malloc(jpeg_len);
		if (*out == NULL) {
			fz_throw(ctx, FZ_ERROR_SYSTEM, "malloc failed");
		}
		memcpy(*out, jpeg_data, jpeg_len);
		*out_len = jpeg_len;
		rc = 0;
	}
	fz_always(ctx) {
		if (device) {
			fz_close_device(ctx, device);
			fz_drop_device(ctx, device);
		}
		if (pixmap) {
			fz_drop_pixmap(ctx, pixmap);
		}
		if (page) {
			fz_drop_page(ctx, page);
		}
		if (doc) {
			fz_drop_document(ctx, doc);
		}
		if (buf) {
			fz_drop_buffer(ctx, buf);
		}
	}
	fz_catch(ctx) {
		const char *msg = fz_caught_message(ctx);
		if (msg && msg[0] != '\0') {
			snprintf(err_buf, err_buf_len, "%s", msg);
		} else {
			snprintf(err_buf, err_buf_len, "mupdf render failed");
		}
		rc = -1;
	}

	fz_drop_context(ctx);

	if (rc != 0 && *out != NULL) {
		free(*out);
		*out = NULL;
		*out_len = 0;
	}

	return rc;
}
*/
import "C"

import (
	"fmt"
	"strings"
	"unsafe"
)

const docRenderJPEGQuality = 75

func renderDocPageJPEG(path string, pageNumber int) ([]byte, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	var out *C.uchar
	var outLen C.size_t
	errBuf := make([]byte, 512)

	rc := C.fb_render_document_page_jpeg(
		cPath,
		C.int(pageNumber),
		C.int(docRenderJPEGQuality),
		&out,
		&outLen,
		(*C.char)(unsafe.Pointer(&errBuf[0])),
		C.size_t(len(errBuf)),
	)
	if rc != 0 {
		msg := strings.TrimRight(string(errBuf), "\x00")
		if msg == "" {
			msg = "mupdf render failed"
		}
		return nil, fmt.Errorf("%s", msg)
	}

	if out == nil || outLen == 0 {
		return nil, fmt.Errorf("mupdf render returned empty jpeg")
	}
	defer C.free(unsafe.Pointer(out))

	return C.GoBytes(unsafe.Pointer(out), C.int(outLen)), nil
}
