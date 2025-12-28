// Package heic implements an HEIC image decoder based on libheif/libde265 compiled to WASM.
package heic

import (
	"errors"
	"image"
	"io"
)

// Errors .
var (
	ErrMemRead  = errors.New("heic: mem read failed")
	ErrMemWrite = errors.New("heic: mem write failed")
	ErrDecode   = errors.New("heic: decode failed")
)

// Decode reads a HEIC image from r and returns it as an image.Image.
func Decode(r io.Reader) (image.Image, error) {
	var err error
	var img image.Image

	if dynamic && !ForceWasmMode {
		img, _, err = decodeDynamic(r, false)
		if err != nil {
			return nil, err
		}
	} else {
		img, _, err = decode(r, false)
		if err != nil {
			return nil, err
		}
	}

	return img, nil
}

// DecodeConfig returns the color model and dimensions of a HEIC image without decoding the entire image.
func DecodeConfig(r io.Reader) (image.Config, error) {
	var err error
	var cfg image.Config

	if dynamic && !ForceWasmMode {
		_, cfg, err = decodeDynamic(r, true)
		if err != nil {
			return image.Config{}, err
		}
	} else {
		_, cfg, err = decode(r, true)
		if err != nil {
			return image.Config{}, err
		}
	}

	return cfg, nil
}

// ForceWasmMode, if true, forces using the WASM-based decoder even if a
// dynamic/shared library is available.
//
// This exists mainly for testing purposes.
//
// It is not safe to change this concurrently with any other use of this
// package.
var ForceWasmMode bool

// Dynamic returns error (if there was any) during opening dynamic/shared library.
func Dynamic() error {
	return dynamicErr
}

// Init initializes wazero runtime and compiles the module.
// This function does nothing if a dynamic/shared library is used and Dynamic() returns nil.
// There is no need to explicitly call this function, first Decode will initialize the runtime.
func Init() {
	if dynamic && dynamicErr == nil {
		return
	}

	initOnce()
}

const (
	alignSize = 16

	heifMaxHeaderSize = 262144

	heifColorspaceUndefined  = 99
	heifColorspaceYCbCr      = 0
	heifColorspaceRGB        = 1
	heifColorspaceMonochrome = 2

	heifChannelY           = 0
	heifChannelCb          = 1
	heifChannelCr          = 2
	heifChannelR           = 3
	heifChannelG           = 4
	heifChannelB           = 5
	heifChannelAlpha       = 6
	heifChannelInterleaved = 10

	heifChromaUndefined       = 99
	heifChromaMonochrome      = 0
	heifChroma420             = 1
	heifChroma422             = 2
	heifChroma444             = 3
	heifChromaInterleavedRGBA = 11

	heifFiletypeYesSupported = 1
)

func alignm(a int) int {
	return (a + (alignSize - 1)) & (^(alignSize - 1))
}

func yCbCrSize(r image.Rectangle, subsampleRatio image.YCbCrSubsampleRatio) (w, h, cw, ch int) {
	w, h = r.Dx(), r.Dy()

	switch subsampleRatio {
	case image.YCbCrSubsampleRatio422:
		cw = (r.Max.X+1)/2 - r.Min.X/2
		ch = h
	case image.YCbCrSubsampleRatio420:
		cw = (r.Max.X+1)/2 - r.Min.X/2
		ch = (r.Max.Y+1)/2 - r.Min.Y/2
	default:
		cw = w
		ch = h
	}

	return
}

func init() {
	image.RegisterFormat("heic", "????ftypheic", Decode, DecodeConfig)
}
