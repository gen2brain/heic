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

func decodeSequence(annexb []byte) ([][]byte, int, int, error) {
	mod := modPool.Get().(*module)
	defer modPool.Put(mod)

	inPtr := mod.Xmalloc(int32(len(annexb)))
	if inPtr == 0 {
		return nil, 0, 0, ErrMemWrite
	}
	defer mod.Xfree(inPtr)
	if !mod.write(inPtr, annexb) {
		return nil, 0, 0, ErrMemWrite
	}

	info := mod.Xmalloc(3 * 4)
	if info == 0 {
		return nil, 0, 0, ErrMemWrite
	}
	defer mod.Xfree(info)

	out := mod.Xdecode_sequence(inPtr, int32(len(annexb)), info)

	width := load32(mod.memory[info:])
	height := load32(mod.memory[info+4:])
	count := load32(mod.memory[info+8:])

	if out == 0 || count == 0 || width == 0 || height == 0 {
		return nil, 0, 0, ErrDecode
	}
	defer mod.Xfree(out)

	frameSize := int(width) * int(height) * 4
	data, ok := mod.read(out, int32(frameSize*int(count)))
	if !ok {
		return nil, 0, 0, ErrMemRead
	}

	frames := make([][]byte, count)
	for i := range frames {
		f := make([]byte, frameSize)
		copy(f, data[i*frameSize:(i+1)*frameSize])
		frames[i] = f
	}

	return frames, int(width), int(height), nil
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
