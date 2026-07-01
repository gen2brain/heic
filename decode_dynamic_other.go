//go:build (linux || darwin) && !(nodynamic || arm || 386 || mips || mipsle || loong64)

package heic

var (
	_heifContextReadFromMemoryWithoutCopy          func(*heifContext, *uint8, uint64, *byte) heifError
	_heifContextGetPrimaryImageHandle              func(*heifContext, **heifImageHandle) heifError
	_heifImageHandleGetPreferredDecodingColorspace func(*heifImageHandle, *int, *int) heifError
	_heifDecodeImage                               func(*heifImageHandle, **heifImage, int, int, *heifDecodingOptions) heifError
)

func heifContextReadFromMemoryWithoutCopy(ctx *heifContext, data []byte) heifError {
	return _heifContextReadFromMemoryWithoutCopy(ctx, &data[0], uint64(len(data)), nil)
}

func heifContextGetPrimaryImageHandle(ctx *heifContext, handle **heifImageHandle) heifError {
	return _heifContextGetPrimaryImageHandle(ctx, handle)
}

func heifImageHandleGetPreferredDecodingColorspace(handle *heifImageHandle, colorspace *int, chroma *int) heifError {
	return _heifImageHandleGetPreferredDecodingColorspace(handle, colorspace, chroma)
}

func heifDecodeImage(handle *heifImageHandle, img **heifImage, colorspace int, chroma int, options *heifDecodingOptions) heifError {
	return _heifDecodeImage(handle, img, colorspace, chroma, options)
}
