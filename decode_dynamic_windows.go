//go:build windows && !(nodynamic || arm || 386 || mips || mipsle || loong64)

package heic

// purego can't return structs on Windows; heif_error comes back via an sret out-param.
var (
	_heifContextReadFromMemoryWithoutCopy          func(*heifError, *heifContext, *uint8, uint64, *byte) uintptr
	_heifContextGetPrimaryImageHandle              func(*heifError, *heifContext, **heifImageHandle) uintptr
	_heifImageHandleGetPreferredDecodingColorspace func(*heifError, *heifImageHandle, *int, *int) uintptr
	_heifDecodeImage                               func(*heifError, *heifImageHandle, **heifImage, int, int, *heifDecodingOptions) uintptr
)

func heifContextReadFromMemoryWithoutCopy(ctx *heifContext, data []byte) heifError {
	var e heifError
	_heifContextReadFromMemoryWithoutCopy(&e, ctx, &data[0], uint64(len(data)), nil)
	return e
}

func heifContextGetPrimaryImageHandle(ctx *heifContext, handle **heifImageHandle) heifError {
	var e heifError
	_heifContextGetPrimaryImageHandle(&e, ctx, handle)
	return e
}

func heifImageHandleGetPreferredDecodingColorspace(handle *heifImageHandle, colorspace *int, chroma *int) heifError {
	var e heifError
	_heifImageHandleGetPreferredDecodingColorspace(&e, handle, colorspace, chroma)
	return e
}

func heifDecodeImage(handle *heifImageHandle, img **heifImage, colorspace int, chroma int, options *heifDecodingOptions) heifError {
	var e heifError
	_heifDecodeImage(&e, handle, img, colorspace, chroma, options)
	return e
}
