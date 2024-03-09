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

	e = heifContextReadFromMemoryWithoutCopy(ctx, data)
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

	cfg.Width = heifImageHandleGetWidth(handle)
	cfg.Height = heifImageHandleGetHeight(handle)
	cfg.ColorModel = color.NRGBAModel

	img := &heifImage{}
	e = heifImageCreate(cfg.Width, cfg.Height, heifColorspaceRgb, heifChromaInterleavedRgba, &img)
	if e.Code != 0 {
		return nil, cfg, ErrDecode
	}
	defer heifImageRelease(img)

	e = heifImageAddPlane(img, heifChannelInterleaved, cfg.Width, cfg.Height, 4)
	if e.Code != 0 {
		return nil, cfg, ErrDecode
	}

	if configOnly {
		return nil, cfg, nil
	}

	options := heifDecodingOptionsAlloc()
	options.ConvertHdrTo8bit = 1
	defer heifDecodingOptionsFree(options)

	e = heifDecodeImage(handle, &img, heifColorspaceRgb, heifChromaInterleavedRgba, options)
	if e.Code != 0 {
		return nil, cfg, ErrDecode
	}

	var stride int
	rgbaData := heifImageGetPlaneReadonly(img, heifChannelInterleaved, &stride)
	size := cfg.Height * stride

	runtime.KeepAlive(data)

	rgba := &image.NRGBA{
		Pix:    make([]uint8, size),
		Stride: stride,
		Rect: image.Rectangle{
			Min: image.Point{X: 0, Y: 0},
			Max: image.Point{X: cfg.Width, Y: cfg.Height},
		},
	}

	copy(rgba.Pix, unsafe.Slice(rgbaData, size))

	return rgba, cfg, nil
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
	purego.RegisterLibFunc(&_heifContextReadFromMemoryWithoutCopy, libheif, "heif_context_read_from_memory_without_copy")
	purego.RegisterLibFunc(&_heifContextSetMaxDecodingThreads, libheif, "heif_context_set_max_decoding_threads")
	purego.RegisterLibFunc(&_heifContextGetPrimaryImageHandle, libheif, "heif_context_get_primary_image_handle")
	purego.RegisterLibFunc(&_heifImageHandleGetWidth, libheif, "heif_image_handle_get_width")
	purego.RegisterLibFunc(&_heifImageHandleGetHeight, libheif, "heif_image_handle_get_height")
	purego.RegisterLibFunc(&_heifImageHandleIsPremultipliedAlpha, libheif, "heif_image_handle_is_premultiplied_alpha")
	purego.RegisterLibFunc(&_heifImageHandleRelease, libheif, "heif_image_handle_release")
	purego.RegisterLibFunc(&_heifDecodingOptionsAlloc, libheif, "heif_decoding_options_alloc")
	purego.RegisterLibFunc(&_heifDecodingOptionsFree, libheif, "heif_decoding_options_free")
	purego.RegisterLibFunc(&_heifImageCreate, libheif, "heif_image_create")
	purego.RegisterLibFunc(&_heifImageAddPlane, libheif, "heif_image_add_plane")
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
	_heifCheckFiletype                    func(*uint8, uint64) int
	_heifContextAlloc                     func() *heifContext
	_heifContextFree                      func(*heifContext)
	_heifContextReadFromMemoryWithoutCopy func(*heifContext, *uint8, uint64, *byte) uintptr
	_heifContextSetMaxDecodingThreads     func(*heifContext, int)
	_heifContextGetPrimaryImageHandle     func(*heifContext, **heifImageHandle) uintptr
	_heifImageHandleGetWidth              func(*heifImageHandle) int
	_heifImageHandleGetHeight             func(*heifImageHandle) int
	_heifImageHandleIsPremultipliedAlpha  func(*heifImageHandle) int
	_heifImageHandleRelease               func(*heifImageHandle)
	_heifDecodingOptionsAlloc             func() *heifDecodingOptions
	_heifDecodingOptionsFree              func(*heifDecodingOptions)
	_heifImageCreate                      func(int, int, int, int, **heifImage) uintptr
	_heifImageAddPlane                    func(*heifImage, int, int, int, int) uintptr
	_heifDecodeImage                      func(*heifImageHandle, **heifImage, int, int, *heifDecodingOptions) uintptr
	_heifImageGetPlaneReadonly            func(*heifImage, int, *int) *uint8
	_heifImageRelease                     func(*heifImage)
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

func heifContextReadFromMemoryWithoutCopy(ctx *heifContext, data []byte) heifError {
	ret := _heifContextReadFromMemoryWithoutCopy(ctx, &data[0], uint64(len(data)), nil)

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

func heifImageCreate(width, height, colorspace, chroma int, img **heifImage) heifError {
	ret := _heifImageCreate(width, height, colorspace, chroma, img)

	return *(*heifError)(unsafe.Pointer(&ret))
}

func heifImageAddPlane(img *heifImage, channel, width, height, bitDepth int) heifError {
	ret := _heifImageAddPlane(img, channel, width, height, bitDepth)

	return *(*heifError)(unsafe.Pointer(&ret))
}

func heifDecodeImage(handle *heifImageHandle, img **heifImage, colorspace int, chroma int, options *heifDecodingOptions) heifError {
	ret := _heifDecodeImage(handle, img, colorspace, chroma, options)

	return *(*heifError)(unsafe.Pointer(&ret))
}

func heifImageGetPlaneReadonly(img *heifImage, channel int, stride *int) *uint8 {
	return _heifImageGetPlaneReadonly(img, channel, stride)
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
