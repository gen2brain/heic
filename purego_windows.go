//go:build windows && !nodynamic

package heic

import (
	"fmt"
	"syscall"
)

const (
	libname = "libheif.dll"
)

func loadLibrary() (uintptr, error) {
	handle, err := syscall.LoadLibrary(libname)
	if err != nil {
		return 0, fmt.Errorf("cannot load library %s: %w", libname, err)
	}

	return uintptr(handle), nil
}
