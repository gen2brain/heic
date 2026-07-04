// Package heic implements an HEIC image decoder based on the pure-Rust heic
// crate compiled to WASM (run via wazero, or transpiled with the wasm2go build
// tag), or libheif/libde265 via a dynamic library.
package heic

//go:generate make -C lib

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
)

// Errors .
var (
	ErrMemRead  = errors.New("heic: mem read failed")
	ErrMemWrite = errors.New("heic: mem write failed")
	ErrDecode   = errors.New("heic: decode failed")
)

// Decode reads a HEIC image from r; for an image sequence it returns the first frame.
func Decode(r io.Reader) (image.Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("heic: read: %w", err)
	}

	if _, ok := parseSequence(data); ok {
		h, err := DecodeAll(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}

		return h.Image[0], nil
	}

	if dynamic && !ForceWasmMode {
		img, _, err := decodeDynamic(bytes.NewReader(data), false)
		return img, err
	}

	img, _, err := decode(bytes.NewReader(data), false)
	return img, err
}

// HEIC holds the decoded frames of a HEIC image sequence and their per-frame delays in seconds.
type HEIC struct {
	Image []image.Image
	Delay []float64
}

// DecodeAll reads a HEIC image sequence from r and returns all frames; a still image yields one frame.
func DecodeAll(r io.Reader) (*HEIC, error) {
	if dynamic && !ForceWasmMode {
		return decodeDynamicAll(r)
	}

	return decodeWasmAll(r)
}

// DecodeConfig returns the color model and dimensions of a HEIC image without decoding the entire image.
func DecodeConfig(r io.Reader) (image.Config, error) {
	data, err := io.ReadAll(io.LimitReader(r, heifMaxHeaderSize))
	if err != nil {
		return image.Config{}, fmt.Errorf("heic: read: %w", err)
	}

	if info, ok := parseSequence(data); ok {
		return image.Config{ColorModel: color.NRGBAModel, Width: info.width, Height: info.height}, nil
	}

	var cfg image.Config
	if dynamic && !ForceWasmMode {
		_, cfg, err = decodeDynamic(bytes.NewReader(data), true)
	} else {
		_, cfg, err = decode(bytes.NewReader(data), true)
	}
	if err != nil {
		return image.Config{}, err
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
	for _, brand := range []string{"heic", "heix", "hevc", "hevx", "msf1"} {
		image.RegisterFormat("heic", "????ftyp"+brand, Decode, DecodeConfig)
	}
}
