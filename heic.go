// Package heic implements an HEIC image decoder based on libheif compiled to WASM.
package heic

import (
	"image"
)

func init() {
	image.RegisterFormat("heic", "????ftypheic", Decode, DecodeConfig)
}
