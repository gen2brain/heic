//go:build !wasm2go

package heic

import (
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"io"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

//go:embed lib/heic.wasm.gz
var heicWasm []byte

type module struct {
	mod    api.Module
	alloc  api.Function
	free   api.Function
	decode api.Function
}

var modPool = sync.Pool{New: func() any { return newModule() }}

func newModule() *module {
	initWasmOnce()

	mod, err := rt.InstantiateModule(context.Background(), cm, mc)
	if err != nil {
		panic(err)
	}

	return &module{
		mod:    mod,
		alloc:  mod.ExportedFunction("malloc"),
		free:   mod.ExportedFunction("free"),
		decode: mod.ExportedFunction("decode"),
	}
}

func decode(r io.Reader, configOnly bool) (image.Image, image.Config, error) {
	var cfg image.Config

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

	m := modPool.Get().(*module)
	defer modPool.Put(m)

	ctx := context.Background()
	mem := m.mod.Memory()

	inSize := len(data)

	res, err := m.alloc.Call(ctx, uint64(inSize))
	if err != nil {
		return nil, cfg, fmt.Errorf("alloc: %w", err)
	}
	inPtr := res[0]
	defer m.free.Call(ctx, inPtr)

	if !mem.Write(uint32(inPtr), data) {
		return nil, cfg, ErrMemWrite
	}

	res, err = m.alloc.Call(ctx, 2*4)
	if err != nil {
		return nil, cfg, fmt.Errorf("alloc: %w", err)
	}
	infoPtr := res[0]
	defer m.free.Call(ctx, infoPtr)

	cfgOnly := uint64(0)
	if configOnly {
		cfgOnly = 1
	}

	res, err = m.decode.Call(ctx, inPtr, uint64(inSize), cfgOnly, infoPtr)
	if err != nil {
		return nil, cfg, fmt.Errorf("decode: %w", err)
	}

	width, ok := mem.ReadUint32Le(uint32(infoPtr))
	if !ok {
		return nil, cfg, ErrMemRead
	}
	height, ok := mem.ReadUint32Le(uint32(infoPtr) + 4)
	if !ok {
		return nil, cfg, ErrMemRead
	}

	cfg.Width = int(width)
	cfg.Height = int(height)
	cfg.ColorModel = color.NRGBAModel

	if configOnly {
		if width == 0 {
			return nil, image.Config{}, ErrDecode
		}
		return nil, cfg, nil
	}

	outPtr := res[0]
	if outPtr == 0 {
		return nil, cfg, ErrDecode
	}
	defer m.free.Call(ctx, outPtr)

	size := int(width) * int(height) * 4
	out, ok := mem.Read(uint32(outPtr), uint32(size))
	if !ok {
		return nil, cfg, ErrMemRead
	}

	img := image.NewNRGBA(image.Rect(0, 0, int(width), int(height)))
	copy(img.Pix, out)

	return img, cfg, nil
}

var (
	rt wazero.Runtime
	cm wazero.CompiledModule
	mc wazero.ModuleConfig

	initWasmOnce = sync.OnceFunc(initialize)
)

func initialize() {
	ctx := context.Background()
	rt = wazero.NewRuntime(ctx)

	r, err := gzip.NewReader(bytes.NewReader(heicWasm))
	if err != nil {
		panic(err)
	}
	defer r.Close()

	var data bytes.Buffer
	if _, err := data.ReadFrom(r); err != nil {
		panic(err)
	}

	cm, err = rt.CompileModule(ctx, data.Bytes())
	if err != nil {
		panic(err)
	}

	mc = wazero.NewModuleConfig().WithName("")
}
