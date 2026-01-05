//go:build linux && !android && !darwin && !(nodynamic || arm || 386 || mips || mipsle)

package heic

import (
	"debug/elf"
	"fmt"
	"os"
	"runtime"

	"github.com/ebitengine/purego"
)

const (
	libname = "libheif.so"
)

func loadLibrary() (uintptr, error) {
	if runtime.GOOS == "linux" && !isDynamicBinary() {
		return 0, fmt.Errorf("not a dynamic binary")
	}

	handle, err := purego.Dlopen(libname, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return 0, fmt.Errorf("cannot load library: %w", err)
	}

	return handle, nil
}

func isDynamicBinary() bool {
	fileName, err := os.Executable()
	if err != nil {
		panic(err)
	}

	fl, err := elf.Open(fileName)
	if err != nil {
		panic(err)
	}

	defer fl.Close()

	_, err = fl.DynamicSymbols()
	if err == nil {
		return true
	}

	return false
}
