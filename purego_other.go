//go:build (!linux && !darwin && !windows) || android || nodynamic || arm || 386 || mips || mipsle || loong64

package heic

import (
	"fmt"
	"image"
	"io"
)

var (
	dynamic    = false
	dynamicErr = fmt.Errorf("heic: dynamic disabled")
)

func decodeDynamic(r io.Reader, configOnly bool) (image.Image, image.Config, error) {
	return nil, image.Config{}, dynamicErr
}

func exifDynamic(data []byte) ([]byte, error) {
	return nil, dynamicErr
}

func loadLibrary() (uintptr, error) {
	return 0, dynamicErr
}
