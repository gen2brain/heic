//go:build windows && !nodynamic

package heic

import (
	"fmt"
	"syscall"
)

const (
	libname = "libheif.dll"
)

func loadLibrary() (handle uintptr, err error) {
	paths := []string{
		libname,
		"heif.dll", // what vcpkg builds & names it
	}
	var firstErr error
	for _, path := range paths {
		sysHandle, err := syscall.LoadLibrary(path)
		if err == nil {
			return uintptr(sysHandle), nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return 0, fmt.Errorf("cannot load library %s: %w", libname, firstErr)
}
