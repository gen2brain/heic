package heic

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
)

func decode(r io.Reader, configOnly bool) (img image.Image, cfg image.Config, err error) {
	mod := newModule()

	defer func() {
		if e := recover(); e != nil {
			if _, ok := e.(procExit); ok {
				img, err = nil, ErrDecode
				return
			}
			panic(e)
		}
	}()

	var data []byte

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

	inPtr := mod.Xmalloc(int32(inSize))
	defer mod.Xfree(inPtr)

	ok := mod.write(inPtr, data)
	if !ok {
		return nil, cfg, ErrMemWrite
	}

	ptr := mod.Xmalloc(4 * 5)
	defer mod.Xfree(ptr)

	widthPtr := ptr
	heightPtr := ptr + 4
	colorspacePtr := ptr + 8
	chromaPtr := ptr + 12
	premultipliedPtr := ptr + 16

	res := mod.Xdecode(inPtr, int32(inSize), 1, widthPtr, heightPtr, colorspacePtr, chromaPtr, premultipliedPtr, 0)
	if res == 0 {
		return nil, cfg, ErrDecode
	}

	width, ok := mod.readUint32(widthPtr)
	if !ok {
		return nil, cfg, ErrMemRead
	}

	height, ok := mod.readUint32(heightPtr)
	if !ok {
		return nil, cfg, ErrMemRead
	}

	colorspace, ok := mod.readUint32(colorspacePtr)
	if !ok {
		return nil, cfg, ErrMemRead
	}

	chroma, ok := mod.readUint32(chromaPtr)
	if !ok {
		return nil, cfg, ErrMemRead
	}

	premultiplied, ok := mod.readUint32(premultipliedPtr)
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

	outPtr := mod.Xmalloc(int32(size))
	defer mod.Xfree(outPtr)

	res = mod.Xdecode(inPtr, int32(inSize), 0, widthPtr, heightPtr, colorspacePtr, chromaPtr, premultipliedPtr, outPtr)
	if res == 0 {
		return nil, cfg, ErrDecode
	}

	out, ok := mod.read(outPtr, int32(size))
	if !ok {
		return nil, cfg, ErrMemRead
	}

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

func newModule() *Module {
	mod := New(&wasiHost{})
	mod.X_initialize()

	return mod
}

func (m *Module) write(ptr int32, data []byte) bool {
	if ptr < 0 || int(ptr)+len(data) > len(m.memory) {
		return false
	}

	copy(m.memory[ptr:], data)

	return true
}

func (m *Module) read(ptr, size int32) ([]byte, bool) {
	if ptr < 0 || size < 0 || int(ptr)+int(size) > len(m.memory) {
		return nil, false
	}

	return m.memory[ptr : ptr+size : ptr+size], true
}

func (m *Module) readUint32(ptr int32) (uint32, bool) {
	if ptr < 0 || int(ptr)+4 > len(m.memory) {
		return 0, false
	}

	return load32(m.memory[ptr:]), true
}

// procExit carries the exit code of a wasi proc_exit call so Decode can turn an
// unexpected module abort into an error instead of crashing.
type procExit struct {
	code int32
}

// errBadf is the wasi EBADF errno, returned from the unused file-I/O imports.
const errBadf = 8

// wasiHost implements the wasi_snapshot_preview1 imports the heif module links
// against. libheif/libde265 only emit diagnostic output (fd_write) and may abort
// (proc_exit); the file-I/O calls are never reached and return EBADF.
type wasiHost struct {
	mod *Module
}

func (h *wasiHost) Init(m any) {
	h.mod = m.(*Module)
}

func (h *wasiHost) Xenviron_get(environ, buf int32) int32 {
	return 0
}

func (h *wasiHost) Xenviron_sizes_get(countPtr, sizePtr int32) int32 {
	store32(h.mod.memory[countPtr:], 0)
	store32(h.mod.memory[sizePtr:], 0)
	return 0
}

func (h *wasiHost) Xfd_close(fd int32) int32 {
	return 0
}

func (h *wasiHost) Xfd_fdstat_get(fd, retPtr int32) int32 {
	return errBadf
}

func (h *wasiHost) Xfd_prestat_dir_name(fd, pathPtr, pathLen int32) int32 {
	return errBadf
}

func (h *wasiHost) Xfd_prestat_get(fd, retPtr int32) int32 {
	return errBadf
}

func (h *wasiHost) Xfd_read(fd, iovs, iovsLen, nreadPtr int32) int32 {
	return errBadf
}

func (h *wasiHost) Xfd_seek(fd int32, offset int64, whence, retPtr int32) int32 {
	return errBadf
}

func (h *wasiHost) Xpath_unlink_file(fd, pathPtr, pathLen int32) int32 {
	return errBadf
}

func (h *wasiHost) Xproc_exit(code int32) {
	panic(procExit{code})
}

func (h *wasiHost) Xfd_write(fd, iovs, iovsLen, nwrittenPtr int32) int32 {
	mem := h.mod.memory

	var dst *os.File
	switch fd {
	case 1:
		dst = os.Stdout
	case 2:
		dst = os.Stderr
	}

	var written uint32
	for i := int32(0); i < iovsLen; i++ {
		ptr := load32(mem[iovs+i*8:])
		length := load32(mem[iovs+i*8+4:])
		if length != 0 && dst != nil {
			dst.Write(mem[ptr : ptr+length])
		}
		written += length
	}

	store32(mem[nwrittenPtr:], written)

	return 0
}
