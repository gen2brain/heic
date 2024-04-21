//go:build darwin && !nodynamic

package heic

import (
	"fmt"

	"github.com/ebitengine/purego"
)

const (
	libname = "libheif.dylib"
)

func loadLibrary() (uintptr, error) {
	handle, err := purego.Dlopen(libname, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return 0, fmt.Errorf("cannot load library: %w", err)
	}

	return uintptr(handle), nil
}
