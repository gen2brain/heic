package heic

import (
	"bytes"
	_ "embed"
	"testing"
)

//go:embed testdata/test_exif.heic
var testHeicExif []byte

func TestDecodeExif(t *testing.T) {
	ex, err := DecodeExif(bytes.NewReader(testHeicExif))
	if err != nil {
		t.Fatal(err)
	}
	if ex.Orientation != 6 {
		t.Errorf("Orientation = %d, want 6", ex.Orientation)
	}
	if ex.Make != "TestCam" {
		t.Errorf("Make = %q, want TestCam", ex.Make)
	}
	if ex.ISOSpeed != 800 {
		t.Errorf("ISOSpeed = %d, want 800", ex.ISOSpeed)
	}
}
