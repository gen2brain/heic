package heic

import (
	"bytes"
	_ "embed"
	"image"
	"image/jpeg"
	"io"
	"os"
	"runtime"
	"strconv"
	"sync"
	"testing"
)

//go:embed testdata/test.heic
var testHeic []byte

//go:embed testdata/test8.heic
var testHeic8 []byte

//go:embed testdata/test12.heic
var testHeic12 []byte

//go:embed testdata/gray.heic
var testGray []byte

func TestDecode(t *testing.T) {
	img, _, err := decode(bytes.NewReader(testHeic), false)
	if err != nil {
		t.Fatal(err)
	}

	w, err := writeCloser()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	err = jpeg.Encode(w, img, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestDecode8(t *testing.T) {
	img, _, err := decode(bytes.NewReader(testHeic8), false)
	if err != nil {
		t.Fatal(err)
	}

	w, err := writeCloser()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	err = jpeg.Encode(w, img, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestDecode12(t *testing.T) {
	img, _, err := decode(bytes.NewReader(testHeic12), false)
	if err != nil {
		t.Fatal(err)
	}

	w, err := writeCloser()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	err = jpeg.Encode(w, img, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestDecodeGray(t *testing.T) {
	img, _, err := decode(bytes.NewReader(testGray), false)
	if err != nil {
		t.Fatal(err)
	}

	w, err := writeCloser()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	err = jpeg.Encode(w, img, nil)
	if err != nil {
		t.Error(err)
	}
}

var inCI, _ = strconv.ParseBool(os.Getenv("CI"))

func requireDynamic(t testing.TB) {
	if err := Dynamic(); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("skipping dynamic library test on Windows in CI; it doesn't work yet: https://github.com/gen2brain/heic/issues/11")
		}
		if inCI {
			t.Fatalf("libheif should be available in CI on %s, but got: %v", runtime.GOOS, err)
		}
		t.Helper()
		t.Skipf("skipping dynamic library test; libheif not available: %v", err)
	}
}

func TestDecodeDynamic(t *testing.T) {
	requireDynamic(t)

	img, _, err := decodeDynamic(bytes.NewReader(testHeic), false)
	if err != nil {
		t.Fatal(err)
	}

	w, err := writeCloser()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	err = jpeg.Encode(w, img, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestDecode8Dynamic(t *testing.T) {
	requireDynamic(t)

	img, _, err := decodeDynamic(bytes.NewReader(testHeic8), false)
	if err != nil {
		t.Fatal(err)
	}

	w, err := writeCloser()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	err = jpeg.Encode(w, img, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestDecode12Dynamic(t *testing.T) {
	requireDynamic(t)

	img, _, err := decodeDynamic(bytes.NewReader(testHeic12), false)
	if err != nil {
		t.Fatal(err)
	}

	w, err := writeCloser()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	err = jpeg.Encode(w, img, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestDecodeGrayDynamic(t *testing.T) {
	requireDynamic(t)

	img, _, err := decodeDynamic(bytes.NewReader(testGray), false)
	if err != nil {
		t.Fatal(err)
	}

	w, err := writeCloser()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	err = jpeg.Encode(w, img, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestImageDecode(t *testing.T) {
	testBothWays(t, func(t *testing.T) {
		img, _, err := image.Decode(bytes.NewReader(testHeic8))
		if err != nil {
			t.Fatal(err)
		}

		err = jpeg.Encode(io.Discard, img, nil)
		if err != nil {
			t.Error(err)
		}
	})
}

func TestDecodeConfig(t *testing.T) {
	testBothWays(t, func(t *testing.T) {
		cfg, err := DecodeConfig(bytes.NewReader(testHeic8))
		if err != nil {
			t.Fatal(err)
		}

		if cfg.Width != 512 {
			t.Errorf("width: got %d, want %d", cfg.Width, 512)
		}

		if cfg.Height != 512 {
			t.Errorf("height: got %d, want %d", cfg.Height, 512)
		}
	})
}

func TestDecodeSync(t *testing.T) {
	wg := sync.WaitGroup{}
	ch := make(chan bool, 2)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			ch <- true
			defer func() { <-ch; wg.Done() }()

			_, _, err := decode(bytes.NewReader(testHeic8), false)
			if err != nil {
				t.Error(err)
				return
			}
		}()
	}

	wg.Wait()
}

func TestDecodeSyncDynamic(t *testing.T) {
	requireDynamic(t)

	wg := sync.WaitGroup{}
	ch := make(chan bool, 2)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			ch <- true
			defer func() { <-ch; wg.Done() }()

			_, _, err := decodeDynamic(bytes.NewReader(testHeic8), false)
			if err != nil {
				t.Error(err)
				return
			}
		}()
	}

	wg.Wait()
}

// smallChunkReader wraps an io.Reader and limits Read calls to small chunks,
// simulating what an io.Reader passed to image.DecodeConfig might legitimately
// do. (The io.Reader contract allows this.)
type smallChunkReader struct{ io.Reader }

func (r smallChunkReader) Read(p []byte) (int, error) {
	const chunkSize = 128
	if len(p) > chunkSize {
		p = p[:chunkSize]
	}
	return r.Reader.Read(p)
}

func TestDecodeConfigViaImagesPackage(t *testing.T) {
	testBothWays(t, func(t *testing.T) {
		cfg, typ, err := image.DecodeConfig(smallChunkReader{bytes.NewReader(testHeic)})
		if err != nil {
			t.Fatal(err)
		}
		if g, w := cfg.Width, 1346; g != w {
			t.Fatalf("invalid width: got %d, want %d", g, w)
		}
		if g, h := cfg.Height, 1346; g != h {
			t.Fatalf("invalid height: got %d, want %d", g, h)
		}
		if typ != "heic" {
			t.Fatalf("invalid type; got %q; want %q", typ, "heic")
		}
	})
}

// testBothWays runs fn in both wasm mode and dynamic library mode, if possible.
func testBothWays(t *testing.T, fn func(t *testing.T)) {
	t.Run("wasm", func(t *testing.T) {
		was := ForceWasmMode
		ForceWasmMode = true
		t.Cleanup(func() { ForceWasmMode = was })
		fn(t)
	})
	t.Run("dynamic", func(t *testing.T) {
		requireDynamic(t)
		fn(t)
	})
}

func BenchmarkDecode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, err := decode(bytes.NewReader(testHeic8), false)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkDecodeDynamic(b *testing.B) {
	requireDynamic(b)

	for i := 0; i < b.N; i++ {
		_, _, err := decodeDynamic(bytes.NewReader(testHeic8), false)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkDecodeConfig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, err := decode(bytes.NewReader(testHeic8), true)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkDecodeConfigDynamic(b *testing.B) {
	requireDynamic(b)

	for i := 0; i < b.N; i++ {
		_, _, err := decodeDynamic(bytes.NewReader(testHeic8), true)
		if err != nil {
			b.Error(err)
		}
	}
}

type discard struct{}

func (d discard) Close() error {
	return nil
}

func (discard) Write(p []byte) (int, error) {
	return len(p), nil
}

var discardCloser io.WriteCloser = discard{}

func writeCloser(s ...string) (io.WriteCloser, error) {
	if len(s) > 0 {
		f, err := os.Create(s[0])
		if err != nil {
			return nil, err
		}

		return f, nil
	}

	return discardCloser, nil
}
