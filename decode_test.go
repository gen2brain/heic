package heic

import (
	"bytes"
	_ "embed"
	"image"
	"image/jpeg"
	"io"
	"testing"
)

//go:embed testdata/test8.heic
var testHeic8 []byte

//go:embed testdata/test12.heic
var testHeic12 []byte

func TestDecode(t *testing.T) {
	img, err := Decode(bytes.NewReader(testHeic8))
	if err != nil {
		t.Fatal(err)
	}

	err = jpeg.Encode(io.Discard, img, nil)
	if err != nil {
		t.Error(err)
	}
}

func TestDecode12(t *testing.T) {
	img, err := Decode(bytes.NewReader(testHeic12))
	if err != nil {
		t.Fatal(err)
	}

	err = jpeg.Encode(io.Discard, img, nil)
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
}

func BenchmarkDecodeHEIC(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, err := decode(bytes.NewReader(testHeic8), false)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkDecodeHEICDynamic(b *testing.B) {
	if !Dynamic() {
		b.Errorf("dynamic/shared library not installed")
		return
	}

	for i := 0; i < b.N; i++ {
		_, _, err := decodeDynamic(bytes.NewReader(testHeic8), false)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkDecodeConfigHEIC(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, err := decode(bytes.NewReader(testHeic8), true)
		if err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkDecodeConfigHEICDynamic(b *testing.B) {
	if !Dynamic() {
		b.Errorf("dynamic/shared library not installed")
		return
	}

	for i := 0; i < b.N; i++ {
		_, _, err := decodeDynamic(bytes.NewReader(testHeic8), true)
		if err != nil {
			b.Error(err)
		}
	}
}
