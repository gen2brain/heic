package heic

import (
	"bytes"
	"encoding/binary"
	"runtime"
	"testing"
)

// box wraps payload in an ISO-BMFF box: 4-byte size, 4-byte type, payload.
func box(typ string, payload []byte) []byte {
	b := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(b[:4], uint32(8+len(payload)))
	copy(b[4:8], typ)
	copy(b[8:], payload)
	return b
}

// forgedSequenceContainer builds a minimal HEIF sequence container whose stsz
// box claims 2^29 samples. Pre-fix, heic.DecodeConfig allocated ~4 GiB for
// this 124-byte file before failing; the parsers must now bound allocations
// by what the input can actually back.
func forgedSequenceContainer() []byte {
	ftyp := make([]byte, 16)
	binary.BigEndian.PutUint32(ftyp[:4], 16)
	copy(ftyp[4:8], "ftyp")
	copy(ftyp[8:12], "msf1")

	stsz := make([]byte, 12)
	binary.BigEndian.PutUint32(stsz[8:12], 1<<29) // forged sample_count
	stbl := box("stbl", box("stsz", stsz))
	mdhd := box("mdhd", make([]byte, 20))
	hdlr := box("hdlr", append(make([]byte, 8), "pict"...))
	mdia := box("mdia", append(append(mdhd, hdlr...), box("minf", stbl)...))
	moov := box("moov", box("trak", mdia))
	return append(ftyp, moov...)
}

func TestParseStszBoundsForgedCounts(t *testing.T) {
	// Per-entry sizes: count must be backed by bytes in the box payload.
	p := make([]byte, 12)
	binary.BigEndian.PutUint32(p[8:12], 1<<30)
	if got := parseStsz(p, 1<<30); len(got) != 0 {
		t.Fatalf("empty payload: got %d entries, want 0", len(got))
	}

	p = make([]byte, 12+2*4)
	binary.BigEndian.PutUint32(p[8:12], 1<<30)
	if got := parseStsz(p, 1<<30); len(got) != 2 {
		t.Fatalf("2 entries present: got %d, want 2", len(got))
	}

	// Uniform table: count capped by maxSamples, not the forged value.
	p = make([]byte, 12)
	binary.BigEndian.PutUint32(p[4:8], 100) // sample_size
	binary.BigEndian.PutUint32(p[8:12], 1<<30)
	if got := parseStsz(p, 16); len(got) != 16 {
		t.Fatalf("uniform forged count: got %d, want 16", len(got))
	}
	binary.BigEndian.PutUint32(p[8:12], 10)
	if got := parseStsz(p, 16); len(got) != 10 {
		t.Fatalf("uniform legit count: got %d, want 10", len(got))
	}
}

func TestParseOffsetsBoundsForgedCounts(t *testing.T) {
	p := make([]byte, 8)
	binary.BigEndian.PutUint32(p[4:8], 1<<30)
	if got := parseOffsets(p, 4); len(got) != 0 {
		t.Fatalf("empty payload: got %d chunks, want 0", len(got))
	}

	p = make([]byte, 8+2*4)
	binary.BigEndian.PutUint32(p[4:8], 1<<30)
	if got := parseOffsets(p, 4); len(got) != 2 {
		t.Fatalf("2 offsets present: got %d, want 2", len(got))
	}
}

func TestParseStscBoundsForgedCounts(t *testing.T) {
	p := make([]byte, 8)
	binary.BigEndian.PutUint32(p[4:8], 1<<30)
	if got := parseStsc(p); len(got) != 0 {
		t.Fatalf("empty payload: got %d values, want 0", len(got))
	}

	p = make([]byte, 8+3*12)
	binary.BigEndian.PutUint32(p[4:8], 1<<30)
	if got := parseStsc(p); len(got) != 6 {
		t.Fatalf("3 entries present: got %d values, want 6", len(got))
	}
}

func TestParseSttsBoundsForgedCounts(t *testing.T) {
	// One entry claiming 2^30 samples: expanded total capped by maxSamples.
	p := make([]byte, 16)
	binary.BigEndian.PutUint32(p[4:8], 1)      // one entry
	binary.BigEndian.PutUint32(p[8:12], 1<<30) // cnt
	binary.BigEndian.PutUint32(p[12:16], 1)    // delta
	if got := parseStts(p, 64); len(got) != 64 {
		t.Fatalf("forged cnt: got %d durations, want 64", len(got))
	}

	// Entry count itself capped by box payload.
	p = make([]byte, 8)
	binary.BigEndian.PutUint32(p[4:8], 1<<30)
	if got := parseStts(p, 1<<30); len(got) != 0 {
		t.Fatalf("empty payload: got %d durations, want 0", len(got))
	}
}

func TestDecodeConfigDoesNotAmplifyForgedSequenceCounts(t *testing.T) {
	data := forgedSequenceContainer()

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	if _, err := DecodeConfig(bytes.NewReader(data)); err == nil {
		t.Fatal("expected decode failure for forged sequence")
	}
	runtime.ReadMemStats(&after)

	if delta := after.TotalAlloc - before.TotalAlloc; delta > 64<<20 {
		t.Fatalf("DecodeConfig allocated %.1f MiB for a %d-byte forged file, want <= 64 MiB",
			float64(delta)/(1<<20), len(data))
	}
}
