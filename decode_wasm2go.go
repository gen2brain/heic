//go:build wasm2go

package heic

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"sync"
)

var modPool = sync.Pool{New: func() any { return newModuleRaw() }}

func decode(r io.Reader, configOnly bool) (image.Image, image.Config, error) {
	var cfg image.Config

	mod := modPool.Get().(*module)
	defer modPool.Put(mod)

	var data []byte
	var err error
	if configOnly {
		data, err = io.ReadAll(io.LimitReader(r, heifMaxHeaderSize))
	} else {
		data, err = io.ReadAll(r)
	}
	if err != nil {
		return nil, cfg, fmt.Errorf("read: %w", err)
	}

	inSize := len(data)

	inPtr := mod.Xmalloc(int32(inSize))
	if inPtr == 0 {
		return nil, cfg, ErrMemWrite
	}
	defer mod.Xfree(inPtr)
	if !mod.write(inPtr, data) {
		return nil, cfg, ErrMemWrite
	}

	info := mod.Xmalloc(2 * 4)
	if info == 0 {
		return nil, cfg, ErrMemWrite
	}
	defer mod.Xfree(info)

	cfgOnly := int32(0)
	if configOnly {
		cfgOnly = 1
	}

	out := mod.Xdecode(inPtr, int32(inSize), cfgOnly, info)

	width := load32(mod.memory[info:])
	height := load32(mod.memory[info+4:])

	cfg.Width = int(width)
	cfg.Height = int(height)
	cfg.ColorModel = color.NRGBAModel

	if configOnly {
		if width == 0 {
			return nil, image.Config{}, ErrDecode
		}
		return nil, cfg, nil
	}

	if out == 0 {
		return nil, cfg, ErrDecode
	}
	defer mod.Xfree(out)

	size := int(width) * int(height) * 4
	pix, ok := mod.read(out, int32(size))
	if !ok {
		return nil, cfg, ErrMemRead
	}

	img := image.NewNRGBA(image.Rect(0, 0, int(width), int(height)))
	copy(img.Pix, pix)

	return img, cfg, nil
}

// exifWasm returns the raw TIFF/EXIF bytes from the HEIC data, or nil when absent.
func exifWasm(data []byte) ([]byte, error) {
	mod := modPool.Get().(*module)
	defer modPool.Put(mod)

	inPtr := mod.Xmalloc(int32(len(data)))
	if inPtr == 0 {
		return nil, ErrMemWrite
	}
	defer mod.Xfree(inPtr)
	if !mod.write(inPtr, data) {
		return nil, ErrMemWrite
	}

	lenPtr := mod.Xmalloc(4)
	if lenPtr == 0 {
		return nil, ErrMemWrite
	}
	defer mod.Xfree(lenPtr)

	out := mod.Xexif(inPtr, int32(len(data)), lenPtr)
	n := load32(mod.memory[lenPtr:])
	if out == 0 || n == 0 {
		return nil, nil
	}
	defer mod.Xfree(out)

	b, ok := mod.read(out, int32(n))
	if !ok {
		return nil, ErrMemRead
	}

	tiff := make([]byte, len(b))
	copy(tiff, b)

	return tiff, nil
}

func (m *module) write(ptr int32, data []byte) bool {
	if ptr < 0 || int(ptr)+len(data) > len(m.memory) {
		return false
	}
	copy(m.memory[ptr:], data)
	return true
}

func (m *module) read(ptr, size int32) ([]byte, bool) {
	if ptr < 0 || size < 0 || int(ptr)+int(size) > len(m.memory) {
		return nil, false
	}
	return m.memory[ptr : ptr+size : ptr+size], true
}
