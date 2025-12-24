package heic

import (
	"bytes"
	"compress/gzip"
	"context"
	"debug/pe"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"runtime"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

//go:embed lib/heif.wasm.gz
var heifWasm []byte

func decode(r io.Reader, configOnly bool) (image.Image, image.Config, error) {
	initOnce()

	var cfg image.Config
	var data []byte

	ctx := context.Background()
	mod, err := rt.InstantiateModule(ctx, cm, mc)
	if err != nil {
		return nil, cfg, err
	}

	defer mod.Close(ctx)

	_alloc := mod.ExportedFunction("malloc")
	_free := mod.ExportedFunction("free")
	_decode := mod.ExportedFunction("decode")

	if configOnly {
		data, err = io.ReadAll(io.LimitReader(r, heifMaxHeaderSize))
		if err != nil {
			return nil, cfg, fmt.Errorf("read: %w", err)
		}
	} else {
		data, err = io.ReadAll(r)
		if err != nil {
			return nil, cfg, fmt.Errorf("read: %w", err)
		}
	}

	inSize := len(data)

	res, err := _alloc.Call(ctx, uint64(inSize))
	if err != nil {
		return nil, cfg, fmt.Errorf("alloc: %w", err)
	}
	inPtr := res[0]
	defer _free.Call(ctx, inPtr)

	ok := mod.Memory().Write(uint32(inPtr), data)
	if !ok {
		return nil, cfg, ErrMemWrite
	}

	res, err = _alloc.Call(ctx, 4*5)
	if err != nil {
		return nil, cfg, fmt.Errorf("alloc: %w", err)
	}
	defer _free.Call(ctx, res[0])

	widthPtr := res[0]
	heightPtr := res[0] + 4
	colorspacePtr := res[0] + 8
	chromaPtr := res[0] + 12
	premultipliedPtr := res[0] + 16

	res, err = _decode.Call(ctx, inPtr, uint64(inSize), 1, widthPtr, heightPtr, colorspacePtr, chromaPtr, premultipliedPtr, 0)
	if err != nil {
		return nil, cfg, fmt.Errorf("decode: %w", err)
	}

	if res[0] == 0 {
		return nil, cfg, ErrDecode
	}

	width, ok := mod.Memory().ReadUint32Le(uint32(widthPtr))
	if !ok {
		return nil, cfg, ErrMemRead
	}

	height, ok := mod.Memory().ReadUint32Le(uint32(heightPtr))
	if !ok {
		return nil, cfg, ErrMemRead
	}

	colorspace, ok := mod.Memory().ReadUint32Le(uint32(colorspacePtr))
	if !ok {
		return nil, cfg, ErrMemRead
	}

	chroma, ok := mod.Memory().ReadUint32Le(uint32(chromaPtr))
	if !ok {
		return nil, cfg, ErrMemRead
	}

	premultiplied, ok := mod.Memory().ReadUint32Le(uint32(premultipliedPtr))
	if !ok {
		return nil, cfg, ErrMemRead
	}

	cfg.Width = int(width)
	cfg.Height = int(height)

	switch colorspace {
	case heifColorspaceYCbCr:
		cfg.ColorModel = color.YCbCrModel
	case heifColorspaceMonochrome:
		cfg.ColorModel = color.GrayModel
	case heifColorspaceRGB:
		if premultiplied == 1 {
			cfg.ColorModel = color.RGBAModel
		} else {
			cfg.ColorModel = color.NRGBAModel
		}
	}

	if configOnly {
		return nil, cfg, nil
	}

	var size int
	var stride int
	var w, h, cw, ch int
	var i0, i1, i2 int
	var subsampleRatio image.YCbCrSubsampleRatio

	rect := image.Rect(0, 0, cfg.Width, cfg.Height)

	switch colorspace {
	case heifColorspaceYCbCr:
		switch chroma {
		case heifChroma420:
			subsampleRatio = image.YCbCrSubsampleRatio420
		case heifChroma422:
			subsampleRatio = image.YCbCrSubsampleRatio422
		case heifChroma444:
			subsampleRatio = image.YCbCrSubsampleRatio444
		default:
			return nil, cfg, fmt.Errorf("unsupported chroma %d", chroma)
		}

		w, h, cw, ch = yCbCrSize(rect, subsampleRatio)
		w = alignm(w)
		cw = alignm(cw)

		i0 = w * h
		i1 = w*h + 1*cw*ch
		i2 = w*h + 2*cw*ch

		size = i2
	case heifColorspaceMonochrome:
		stride = alignm(cfg.Width * 1)
		size = cfg.Height * stride
	case heifColorspaceRGB:
		stride = alignm(cfg.Width * 4)
		size = cfg.Height * stride
	default:
		return nil, cfg, fmt.Errorf("unsupported colorspace %d", colorspace)
	}

	res, err = _alloc.Call(ctx, uint64(size))
	if err != nil {
		return nil, cfg, fmt.Errorf("alloc: %w", err)
	}
	outPtr := res[0]
	defer _free.Call(ctx, outPtr)

	res, err = _decode.Call(ctx, inPtr, uint64(inSize), 0, widthPtr, heightPtr, colorspacePtr, chromaPtr, premultipliedPtr, outPtr)
	if err != nil {
		return nil, cfg, fmt.Errorf("decode: %w", err)
	}

	if res[0] == 0 {
		return nil, cfg, ErrDecode
	}

	out, ok := mod.Memory().Read(uint32(outPtr), uint32(size))
	if !ok {
		return nil, cfg, ErrMemRead
	}

	var img image.Image

	switch colorspace {
	case heifColorspaceYCbCr:
		img = &image.YCbCr{
			Y:              out[:i0:i0],
			Cb:             out[i0:i1:i1],
			Cr:             out[i1:i2:i2],
			SubsampleRatio: subsampleRatio,
			YStride:        w,
			CStride:        cw,
			Rect:           rect,
		}
	case heifColorspaceMonochrome:
		img = &image.Gray{
			Pix:    out,
			Stride: stride,
			Rect:   rect,
		}
	case heifColorspaceRGB:
		if premultiplied == 1 {
			img = &image.RGBA{
				Pix:    out,
				Stride: stride,
				Rect:   rect,
			}
		} else {
			img = &image.NRGBA{
				Pix:    out,
				Stride: stride,
				Rect:   rect,
			}
		}
	}

	return img, cfg, nil
}

var (
	rt wazero.Runtime
	cm wazero.CompiledModule
	mc wazero.ModuleConfig

	initOnce = sync.OnceFunc(initialize)
)

func initialize() {
	ctx := context.Background()
	rt = wazero.NewRuntime(ctx)

	r, err := gzip.NewReader(bytes.NewReader(heifWasm))
	if err != nil {
		panic(err)
	}
	defer r.Close()

	var data bytes.Buffer
	_, err = data.ReadFrom(r)
	if err != nil {
		panic(err)
	}

	cm, err = rt.CompileModule(ctx, data.Bytes())
	if err != nil {
		panic(err)
	}

	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	if runtime.GOOS == "windows" && isWindowsGUI() {
		mc = wazero.NewModuleConfig().WithStderr(io.Discard).WithStdout(io.Discard)
	} else {
		mc = wazero.NewModuleConfig().WithStderr(os.Stderr).WithStdout(os.Stdout)
	}
}

func isWindowsGUI() bool {
	const imageSubsystemWindowsGui = 2

	fileName, err := os.Executable()
	if err != nil {
		return false
	}

	fl, err := pe.Open(fileName)
	if err != nil {
		return false
	}

	defer fl.Close()

	var subsystem uint16
	if header, ok := fl.OptionalHeader.(*pe.OptionalHeader64); ok {
		subsystem = header.Subsystem
	} else if header, ok := fl.OptionalHeader.(*pe.OptionalHeader32); ok {
		subsystem = header.Subsystem
	}

	if subsystem == imageSubsystemWindowsGui {
		return true
	}

	return false
}
