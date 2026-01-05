//go:build (linux || darwin || windows) && !(nodynamic || arm || 386 || mips || mipsle)

package heic

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
)

func decodeDynamic(r io.Reader, configOnly bool) (image.Image, image.Config, error) {
	var err error
	var cfg image.Config
	var data []byte

	if configOnly {
		data, err = io.ReadAll(io.LimitReader(r, heifMaxHeaderSize))
		if err != nil {
			return nil, cfg, fmt.Errorf("read: %w", err)
		}
	} else {
		data, err = io.ReadAll(r)
		if err != nil {
			return nil, cfg, fmt.Errorf("read: %w", err)
		}
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

	handle := new(heifImageHandle)

	e = heifContextGetPrimaryImageHandle(ctx, &handle)
	if e.Code != 0 {
		return nil, cfg, ErrDecode
	}
	defer heifImageHandleRelease(handle)

	cfg.Width = heifImageHandleGetWidth(handle)
	cfg.Height = heifImageHandleGetHeight(handle)

	isPremultiplied := heifImageHandleIsPremultipliedAlpha(handle)

	var colorspace, chroma int
	if versionMajor == 1 && versionMinor >= 17 {
		e = heifImageHandleGetPreferredDecodingColorspace(handle, &colorspace, &chroma)
		if e.Code != 0 {
			return nil, cfg, ErrDecode
		}

		if colorspace == heifColorspaceUndefined || chroma == heifChromaUndefined {
			colorspace = heifColorspaceYCbCr
			chroma = heifChroma420
			cfg.ColorModel = color.YCbCrModel
		}
		if colorspace == heifColorspaceRGB {
			chroma = heifChromaInterleavedRGBA
			if isPremultiplied {
				cfg.ColorModel = color.RGBAModel
			} else {
				cfg.ColorModel = color.NRGBAModel
			}
		}
	} else {
		colorspace = heifColorspaceYCbCr
		chroma = heifChroma420
		cfg.ColorModel = color.YCbCrModel
	}

	if configOnly {
		return nil, cfg, nil
	}

	options := heifDecodingOptionsAlloc()
	options.ConvertHdrTo8bit = 1
	defer heifDecodingOptionsFree(options)

	heifImg := new(heifImage)

	e = heifDecodeImage(handle, &heifImg, colorspace, chroma, options)
	if e.Code != 0 {
		return nil, cfg, ErrDecode
	}

	var img image.Image
	rect := image.Rect(0, 0, cfg.Width, cfg.Height)

	switch colorspace {
	case heifColorspaceYCbCr:
		var subsampleRatio image.YCbCrSubsampleRatio
		switch chroma {
		case heifChroma420:
			subsampleRatio = image.YCbCrSubsampleRatio420
		case heifChroma422:
			subsampleRatio = image.YCbCrSubsampleRatio422
		case heifChroma444:
			subsampleRatio = image.YCbCrSubsampleRatio444
		}

		var yStride, uStride int
		y := heifImageGetPlaneReadonly(heifImg, heifChannelY, &yStride)
		cb := heifImageGetPlaneReadonly(heifImg, heifChannelCb, &uStride)
		cr := heifImageGetPlaneReadonly(heifImg, heifChannelCr, &uStride)

		_, _, _, ch := yCbCrSize(rect, subsampleRatio)
		i0 := yStride * cfg.Height
		i1 := yStride*cfg.Height + 1*uStride*ch
		i2 := yStride*cfg.Height + 2*uStride*ch
		b := make([]byte, i2)

		i := &image.YCbCr{
			Y:              b[:i0:i0],
			Cb:             b[i0:i1:i1],
			Cr:             b[i1:i2:i2],
			SubsampleRatio: subsampleRatio,
			YStride:        yStride,
			CStride:        uStride,
			Rect:           rect,
		}

		copy(i.Y, unsafe.Slice(y, yStride*cfg.Height))
		copy(i.Cb, unsafe.Slice(cb, uStride*ch))
		copy(i.Cr, unsafe.Slice(cr, uStride*ch))

		img = i
	case heifColorspaceMonochrome:
		var stride int
		grayData := heifImageGetPlaneReadonly(heifImg, heifChannelY, &stride)
		size := cfg.Height * stride

		i := &image.Gray{
			Pix:    make([]uint8, size),
			Stride: stride,
			Rect:   rect,
		}

		copy(i.Pix, unsafe.Slice(grayData, size))
		img = i
	case heifColorspaceRGB:
		var stride int
		rgbaData := heifImageGetPlaneReadonly(heifImg, heifChannelInterleaved, &stride)
		size := cfg.Height * stride

		if isPremultiplied {
			i := &image.RGBA{
				Pix:    make([]uint8, size),
				Stride: stride,
				Rect:   rect,
			}

			copy(i.Pix, unsafe.Slice(rgbaData, size))
			img = i
		} else {
			i := &image.NRGBA{
				Pix:    make([]uint8, size),
				Stride: stride,
				Rect:   rect,
			}

			copy(i.Pix, unsafe.Slice(rgbaData, size))
			img = i
		}
	default:
		return nil, cfg, fmt.Errorf("unsupported colorspace %d", colorspace)
	}

	runtime.KeepAlive(data)

	return img, cfg, nil
}

func init() {
	if runtime.GOOS == "windows" {
		dynamic = false
		dynamicErr = fmt.Errorf("dynamic library loading not supported on windows yet; see https://github.com/gen2brain/heic/issues/11")
		return
	}

	var err error
	defer func() {
		if r := recover(); r != nil {
			dynamic = false
			dynamicErr = fmt.Errorf("%v", r)
		}
	}()

	libheif, err = loadLibrary()
	if err == nil {
		dynamic = true
	} else {
		dynamicErr = err

		return
	}

	purego.RegisterLibFunc(&_heifGetVersionNumberMajor, libheif, "heif_get_version_number_major")
	purego.RegisterLibFunc(&_heifGetVersionNumberMinor, libheif, "heif_get_version_number_minor")

	versionMajor = heifGetVersionNumberMajor()
	versionMinor = heifGetVersionNumberMinor()

	if versionMajor == 1 && versionMinor >= 17 {
		purego.RegisterLibFunc(&_heifImageHandleGetPreferredDecodingColorspace, libheif, "heif_image_handle_get_preferred_decoding_colorspace")
	}

	purego.RegisterLibFunc(&_heifCheckFiletype, libheif, "heif_check_filetype")
	purego.RegisterLibFunc(&_heifContextAlloc, libheif, "heif_context_alloc")
	purego.RegisterLibFunc(&_heifContextFree, libheif, "heif_context_free")
	purego.RegisterLibFunc(&_heifContextReadFromMemoryWithoutCopy, libheif, "heif_context_read_from_memory_without_copy")
	purego.RegisterLibFunc(&_heifContextGetPrimaryImageHandle, libheif, "heif_context_get_primary_image_handle")
	purego.RegisterLibFunc(&_heifImageHandleGetWidth, libheif, "heif_image_handle_get_width")
	purego.RegisterLibFunc(&_heifImageHandleGetHeight, libheif, "heif_image_handle_get_height")
	purego.RegisterLibFunc(&_heifImageHandleIsPremultipliedAlpha, libheif, "heif_image_handle_is_premultiplied_alpha")
	purego.RegisterLibFunc(&_heifImageHandleRelease, libheif, "heif_image_handle_release")
	purego.RegisterLibFunc(&_heifDecodingOptionsAlloc, libheif, "heif_decoding_options_alloc")
	purego.RegisterLibFunc(&_heifDecodingOptionsFree, libheif, "heif_decoding_options_free")
	purego.RegisterLibFunc(&_heifDecodeImage, libheif, "heif_decode_image")
	purego.RegisterLibFunc(&_heifImageGetPlaneReadonly, libheif, "heif_image_get_plane_readonly")
}

var (
	libheif uintptr

	dynamic    bool
	dynamicErr error

	versionMajor int
	versionMinor int
)

var (
	_heifGetVersionNumberMajor                     func() uint32
	_heifGetVersionNumberMinor                     func() uint32
	_heifCheckFiletype                             func(*uint8, uint64) int
	_heifContextAlloc                              func() *heifContext
	_heifContextFree                               func(*heifContext)
	_heifContextReadFromMemoryWithoutCopy          func(*heifContext, *uint8, uint64, *byte) uintptr
	_heifContextGetPrimaryImageHandle              func(*heifContext, **heifImageHandle) uintptr
	_heifImageHandleGetWidth                       func(*heifImageHandle) int
	_heifImageHandleGetHeight                      func(*heifImageHandle) int
	_heifImageHandleIsPremultipliedAlpha           func(*heifImageHandle) int
	_heifImageHandleGetPreferredDecodingColorspace func(*heifImageHandle, *int, *int) uintptr
	_heifImageHandleRelease                        func(*heifImageHandle)
	_heifDecodingOptionsAlloc                      func() *heifDecodingOptions
	_heifDecodingOptionsFree                       func(*heifDecodingOptions)
	_heifDecodeImage                               func(*heifImageHandle, **heifImage, int, int, *heifDecodingOptions) uintptr
	_heifImageGetPlaneReadonly                     func(*heifImage, int, *int) *uint8
)

func heifGetVersionNumberMajor() int {
	return int(_heifGetVersionNumberMajor())
}

func heifGetVersionNumberMinor() int {
	return int(_heifGetVersionNumberMinor())
}

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

func heifImageHandleGetPreferredDecodingColorspace(handle *heifImageHandle, colorspace *int, chroma *int) heifError {
	ret := _heifImageHandleGetPreferredDecodingColorspace(handle, colorspace, chroma)

	return *(*heifError)(unsafe.Pointer(&ret))
}

func heifImageHandleRelease(handle *heifImageHandle) {
	_heifImageHandleRelease(handle)
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
	return _heifImageGetPlaneReadonly(img, channel, stride)
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
