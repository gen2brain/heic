package heic

import (
	"image"
	"image/color"
	"io"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
)

func decodeDynamic(r io.Reader, configOnly bool) (image.Image, image.Config, error) {
	var cfg image.Config

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, cfg, err
	}

	check := heifCheckFiletype(data)
	if check != heifFiletypeYesSupported {
		return nil, cfg, ErrDecode
	}

	ctx := heifContextAlloc()
	defer heifContextFree(ctx)

	var e heifError

	e = heifContextReadFromMemory(ctx, data)
	if e.Code != 0 {
		return nil, cfg, ErrDecode
	}

	heifContextSetMaxDecodingThreads(ctx, runtime.NumCPU())

	handle := &heifImageHandle{}
	e = heifContextGetPrimaryImageHandle(ctx, &handle)
	if e.Code != 0 {
		return nil, cfg, ErrDecode
	}
	defer heifImageHandleRelease(handle)

	isPremultipliedAlpha := heifImageHandleIsPremultipliedAlpha(handle)

	cfg.Width = heifImageHandleGetWidth(handle)
	cfg.Height = heifImageHandleGetHeight(handle)

	cfg.ColorModel = color.NRGBAModel
	if isPremultipliedAlpha {
		cfg.ColorModel = color.RGBAModel
	}

	if configOnly {
		return nil, cfg, nil
	}

	options := heifDecodingOptionsAlloc()
	options.ConvertHdrTo8bit = 1
	defer heifDecodingOptionsFree(options)

	img := &heifImage{}
	e = heifDecodeImage(handle, &img, heifColorspaceRgb, heifChromaInterleavedRgba, options)
	if e.Code != 0 {
		return nil, cfg, ErrDecode
	}
	defer heifImageRelease(img)

	rgbaData := heifImageGetPlaneReadonly(img, heifChannelInterleaved, nil)
	size := cfg.Width * cfg.Height * 4

	runtime.KeepAlive(data)

	if isPremultipliedAlpha {
		rgba := image.NewRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))
		copy(rgba.Pix, unsafe.Slice(rgbaData, size))

		return rgba, cfg, nil
	} else {
		rgba := image.NewNRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))
		copy(rgba.Pix, unsafe.Slice(rgbaData, size))

		return rgba, cfg, nil
	}
}

func init() {
	var err error

	libheif, err = loadLibrary()
	if err == nil {
		dynamic = true
	} else {
		return
	}

	purego.RegisterLibFunc(&_heifCheckFiletype, libheif, "heif_check_filetype")
	purego.RegisterLibFunc(&_heifContextAlloc, libheif, "heif_context_alloc")
	purego.RegisterLibFunc(&_heifContextFree, libheif, "heif_context_free")
	purego.RegisterLibFunc(&_heifContextReadFromMemory, libheif, "heif_context_read_from_memory")
	purego.RegisterLibFunc(&_heifContextSetMaxDecodingThreads, libheif, "heif_context_set_max_decoding_threads")
	purego.RegisterLibFunc(&_heifContextGetPrimaryImageHandle, libheif, "heif_context_get_primary_image_handle")
	purego.RegisterLibFunc(&_heifImageHandleGetWidth, libheif, "heif_image_handle_get_width")
	purego.RegisterLibFunc(&_heifImageHandleGetHeight, libheif, "heif_image_handle_get_height")
	purego.RegisterLibFunc(&_heifImageHandleIsPremultipliedAlpha, libheif, "heif_image_handle_is_premultiplied_alpha")
	purego.RegisterLibFunc(&_heifImageHandleRelease, libheif, "heif_image_handle_release")
	purego.RegisterLibFunc(&_heifDecodingOptionsAlloc, libheif, "heif_decoding_options_alloc")
	purego.RegisterLibFunc(&_heifDecodingOptionsFree, libheif, "heif_decoding_options_free")
	purego.RegisterLibFunc(&_heifDecodeImage, libheif, "heif_decode_image")
	purego.RegisterLibFunc(&_heifImageGetPlaneReadonly, libheif, "heif_image_get_plane_readonly")
	purego.RegisterLibFunc(&_heifImageRelease, libheif, "heif_image_release")
}

const (
	heifColorspaceRgb         = 1
	heifChannelInterleaved    = 10
	heifChromaInterleavedRgba = 11
	heifFiletypeYesSupported  = 1
)

var (
	libheif uintptr
	dynamic bool
)

var (
	_heifCheckFiletype                   func(*uint8, uint64) int
	_heifContextAlloc                    func() *heifContext
	_heifContextFree                     func(*heifContext)
	_heifContextReadFromMemory           func(*heifContext, *uint8, uint64, *byte) uintptr
	_heifContextSetMaxDecodingThreads    func(*heifContext, int)
	_heifContextGetPrimaryImageHandle    func(*heifContext, **heifImageHandle) uintptr
	_heifImageHandleGetWidth             func(*heifImageHandle) int
	_heifImageHandleGetHeight            func(*heifImageHandle) int
	_heifImageHandleIsPremultipliedAlpha func(*heifImageHandle) int
	_heifImageHandleRelease              func(*heifImageHandle)
	_heifDecodingOptionsAlloc            func() *heifDecodingOptions
	_heifDecodingOptionsFree             func(*heifDecodingOptions)
	_heifDecodeImage                     func(*heifImageHandle, **heifImage, int, int, *heifDecodingOptions) uintptr
	_heifImageGetPlaneReadonly           func(*heifImage, uint32, *int) *uint8
	_heifImageRelease                    func(*heifImage)
)

func heifCheckFiletype(data []byte) int {
	return _heifCheckFiletype(&data[0], uint64(len(data)))
}

func heifContextAlloc() *heifContext {
	return _heifContextAlloc()
}

func heifContextFree(ctx *heifContext) {
	_heifContextFree(ctx)
}

func heifContextReadFromMemory(ctx *heifContext, data []byte) heifError {
	ret := _heifContextReadFromMemory(ctx, &data[0], uint64(len(data)), nil)

	return *(*heifError)(unsafe.Pointer(&ret))
}

func heifContextSetMaxDecodingThreads(ctx *heifContext, threads int) {
	_heifContextSetMaxDecodingThreads(ctx, threads)
}

func heifContextGetPrimaryImageHandle(ctx *heifContext, handle **heifImageHandle) heifError {
	ret := _heifContextGetPrimaryImageHandle(ctx, handle)

	return *(*heifError)(unsafe.Pointer(&ret))
}

func heifImageHandleGetWidth(handle *heifImageHandle) int {
	return _heifImageHandleGetWidth(handle)
}

func heifImageHandleGetHeight(handle *heifImageHandle) int {
	return _heifImageHandleGetHeight(handle)
}

func heifImageHandleIsPremultipliedAlpha(handle *heifImageHandle) bool {
	ret := _heifImageHandleIsPremultipliedAlpha(handle)

	return ret != 0
}

func heifImageHandleRelease(ctx *heifImageHandle) {
	_heifImageHandleRelease(ctx)
}

func heifDecodingOptionsAlloc() *heifDecodingOptions {
	return _heifDecodingOptionsAlloc()
}

func heifDecodingOptionsFree(options *heifDecodingOptions) {
	_heifDecodingOptionsFree(options)
}

func heifDecodeImage(handle *heifImageHandle, img **heifImage, colorspace int, chroma int, options *heifDecodingOptions) heifError {
	ret := _heifDecodeImage(handle, img, colorspace, chroma, options)

	return *(*heifError)(unsafe.Pointer(&ret))
}

func heifImageGetPlaneReadonly(img *heifImage, channel int, stride *int) *uint8 {
	return _heifImageGetPlaneReadonly(img, uint32(channel), stride)
}

func heifImageRelease(img *heifImage) {
	_heifImageRelease(img)
}

type heifContext struct{}
type heifImageHandle struct{}
type heifImage struct{}

type heifError struct {
	Code    uint32
	Subcode uint32
	Message *int8
}

type heifDecodingOptions struct {
	Version               uint8
	IgnoreTransformations uint8
	StartProgress         *[0]byte
	OnProgress            *[0]byte
	EndProgress           *[0]byte
	ProgressUserData      *byte
	ConvertHdrTo8bit      uint8
	StrictDecoding        uint8
	DecoderId             *int8
}
