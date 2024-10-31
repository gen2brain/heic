package heic

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
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

func TestDecodeDynamic(t *testing.T) {
	if err := Dynamic(); err != nil {
		fmt.Println(err)
		t.Skip()
	}

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
	if err := Dynamic(); err != nil {
		fmt.Println(err)
		t.Skip()
	}

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
	if err := Dynamic(); err != nil {
		fmt.Println(err)
		t.Skip()
	}

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
	if err := Dynamic(); err != nil {
		fmt.Println(err)
		t.Skip()
	}

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
	img, _, err := image.Decode(bytes.NewReader(testHeic8))
	if err != nil {
		t.Fatal(err)
	}

	err = jpeg.Encode(io.Discard, img, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestDecodeConfig(t *testing.T) {
	_, cfg, err := decode(bytes.NewReader(testHeic8), true)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Width != 512 {
		t.Errorf("width: got %d, want %d", cfg.Width, 512)
	}

	if cfg.Height != 512 {
		t.Errorf("height: got %d, want %d", cfg.Height, 512)
	}
}

func TestDecodeConfigDynamic(t *testing.T) {
	if err := Dynamic(); err != nil {
		fmt.Println(err)
		t.Skip()
	}

	_, cfg, err := decodeDynamic(bytes.NewReader(testHeic8), true)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Width != 512 {
		t.Errorf("width: got %d, want %d", cfg.Width, 512)
	}

	if cfg.Height != 512 {
		t.Errorf("height: got %d, want %d", cfg.Height, 512)
	}
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
func BenchmarkDecode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, err := decode(bytes.NewReader(testHeic8), false)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkDecodeDynamic(b *testing.B) {
	if err := Dynamic(); err != nil {
		fmt.Println(err)
		b.Skip()
	}

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
	if err := Dynamic(); err != nil {
		fmt.Println(err)
		b.Skip()
	}

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
