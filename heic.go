// Package heic implements an HEIC image decoder based on libheif compiled to WASM.
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

	if dynamic {
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

	if dynamic {
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

// Dynamic returns error (if there was any) during opening dynamic/shared library.
func Dynamic() error {
	return dynamicErr
}

func init() {
	image.RegisterFormat("heic", "????ftypheic", Decode, DecodeConfig)
}
