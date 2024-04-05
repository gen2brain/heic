//go:build !unix && !darwin && !windows

package heic

import (
	"fmt"
	"image"
	"io"
	"runtime"
)

var (
	dynamic    = false
	dynamicErr = fmt.Errorf("heic: unsupported os: %s", runtime.GOOS)
)

func decodeDynamic(r io.Reader, configOnly bool) (image.Image, image.Config, error) {
	return nil, image.Config{}, dynamicErr
}

func loadLibrary() (uintptr, error) {
	return 0, dynamicErr
}
