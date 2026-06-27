package heic

import (
	"bytes"
	"os"
	"testing"
)

func TestDecodeExif(t *testing.T) {
	data, err := os.ReadFile("testdata/exif.heic")
	if err != nil {
		t.Fatal(err)
	}

	for _, wasm := range []bool{false, true} {
		name := "wasm"
		if !wasm {
			name = "dynamic"
			if Dynamic() != nil {
				continue // libheif not available
			}
		}

		t.Run(name, func(t *testing.T) {
			ForceWasmMode = wasm
			defer func() { ForceWasmMode = false }()

			ex, err := DecodeExif(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if ex.Orientation != 6 {
				t.Errorf("Orientation = %d, want 6", ex.Orientation)
			}
			if ex.Make != "TestCam" {
				t.Errorf("Make = %q, want TestCam", ex.Make)
			}
			if ex.Model != "HeicEXIF" {
				t.Errorf("Model = %q, want HeicEXIF", ex.Model)
			}
			if ex.Software != "cbconvert" {
				t.Errorf("Software = %q, want cbconvert", ex.Software)
			}
		})
	}
}

func TestDecodeExifNone(t *testing.T) {
	data, err := os.ReadFile("testdata/test12.heic")
	if err != nil {
		t.Fatal(err)
	}

	for _, wasm := range []bool{false, true} {
		if !wasm && Dynamic() != nil {
			continue
		}

		ForceWasmMode = wasm
		if _, err := DecodeExif(bytes.NewReader(data)); err != ErrNoExif {
			t.Errorf("wasm=%v: err = %v, want ErrNoExif", wasm, err)
		}
		ForceWasmMode = false
	}
}
