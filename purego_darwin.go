//go:build darwin && !nodynamic

package heic

import (
	"github.com/ebitengine/purego"
)

const (
	libname = "libheif.dylib"
)

func loadLibrary() (handle uintptr, err error) {
	for _, path := range []string{
		libname,
		"/opt/homebrew/lib/libheif.dylib",
	} {
		handle, err = purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err == nil {
			return handle, nil
		}
	}
	return 0, err
}
