//go:build (!unix && !darwin && !windows) || nodynamic

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

func loadLibrary() (uintptr, error) {
	return 0, dynamicErr
}
