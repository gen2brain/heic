package heic

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"io"
)

// decodeWasmAll decodes a HEIC image sequence via the WASM decoder, or a single frame when there is no sequence.
func decodeWasmAll(r io.Reader) (*HEIC, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	if info, ok := parseSequence(data); ok {
		if frames, w, h, err := decodeSequence(info.annexB(data)); err == nil && len(frames) > 0 {
			out := &HEIC{}
			for i, f := range frames {
				img := image.NewNRGBA(image.Rect(0, 0, w, h))
				copy(img.Pix, f)
				out.Image = append(out.Image, img)

				// Frames are display-ordered; durations pair by index (exact for constant frame rate).
				delay := 0.0
				if i < len(info.durations) {
					delay = float64(info.durations[i]) / float64(info.timescale)
				}
				out.Delay = append(out.Delay, delay)
			}

			return out, nil
		}
	}

	img, _, err := decode(bytes.NewReader(data), false)
	if err != nil {
		return nil, err
	}

	return &HEIC{Image: []image.Image{img}, Delay: []float64{0}}, nil
}

type seqSample struct {
	offset int64
	size   int64
}

type seqInfo struct {
	width, height int
	timescale     uint32
	nalLenSize    int
	params        [][]byte
	samples       []seqSample
	durations     []uint32
}

// parseSequence extracts the visual (pict) track's sample table from the moov box, or reports false.
func parseSequence(data []byte) (*seqInfo, bool) {
	var info *seqInfo

	eachBox(data, func(typ string, moov []byte) bool {
		if typ != "moov" {
			return true
		}
		eachBox(moov, func(typ string, trak []byte) bool {
			if typ != "trak" {
				return true
			}
			if ti, ok := parseTrak(trak, data); ok {
				info = ti
				return false
			}
			return true
		})
		return false
	})

	if info == nil || len(info.samples) == 0 || len(info.params) == 0 {
		return nil, false
	}

	return info, true
}

func parseTrak(trak, data []byte) (*seqInfo, bool) {
	info := &seqInfo{nalLenSize: 4}
	isPict := false

	var stbl []byte

	eachBox(trak, func(typ string, mdia []byte) bool {
		if typ != "mdia" {
			return true
		}
		eachBox(mdia, func(typ string, p []byte) bool {
			switch typ {
			case "mdhd":
				if len(p) >= 20 {
					if p[0] == 0 {
						info.timescale = binary.BigEndian.Uint32(p[12:16])
					} else if len(p) >= 28 {
						info.timescale = binary.BigEndian.Uint32(p[20:24])
					}
				}
			case "hdlr":
				if len(p) >= 12 && string(p[8:12]) == "pict" {
					isPict = true
				}
			case "minf":
				eachBox(p, func(typ string, q []byte) bool {
					if typ == "stbl" {
						stbl = q
						return false
					}
					return true
				})
			}
			return true
		})
		return false
	})

	if !isPict || stbl == nil {
		return nil, false
	}

	var sizes []int64
	var chunks []int64
	var stsc []int

	eachBox(stbl, func(typ string, p []byte) bool {
		switch typ {
		case "stsd":
			parseStsd(p, info)
		case "stsz":
			sizes = parseStsz(p, len(data))
		case "stco":
			chunks = parseOffsets(p, 4)
		case "co64":
			chunks = parseOffsets(p, 8)
		case "stsc":
			stsc = parseStsc(p)
		case "stts":
			info.durations = parseStts(p, len(data))
		}
		return true
	})

	if len(sizes) == 0 || len(chunks) == 0 {
		return nil, false
	}

	info.samples = sampleOffsets(chunks, stsc, sizes)
	if info.timescale == 0 {
		info.timescale = 1
	}

	return info, true
}

// parseStsd reads the visual sample entry's dimensions and its hvcC decoder configuration.
func parseStsd(p []byte, info *seqInfo) {
	if len(p) < 16 {
		return
	}

	entry := p[8:]
	if len(entry) < 8 {
		return
	}
	entry = entry[8:] // skip sample entry box size + type

	if len(entry) >= 28 {
		info.width = int(binary.BigEndian.Uint16(entry[24:26]))
		info.height = int(binary.BigEndian.Uint16(entry[26:28]))
	}
	if len(entry) < 78 {
		return
	}

	eachBox(entry[78:], func(typ string, b []byte) bool {
		if typ == "hvcC" {
			if n, params, ok := parseHvcC(b); ok {
				info.nalLenSize = n
				info.params = params
			}
			return false
		}
		return true
	})
}

func parseHvcC(b []byte) (nalLenSize int, params [][]byte, ok bool) {
	if len(b) < 23 {
		return 0, nil, false
	}

	nalLenSize = int(b[21]&0x3) + 1
	numArrays := int(b[22])
	off := 23

	for a := 0; a < numArrays; a++ {
		if off+3 > len(b) {
			return 0, nil, false
		}
		numNalus := int(binary.BigEndian.Uint16(b[off+1 : off+3]))
		off += 3

		for n := 0; n < numNalus; n++ {
			if off+2 > len(b) {
				return 0, nil, false
			}
			l := int(binary.BigEndian.Uint16(b[off : off+2]))
			off += 2
			if off+l > len(b) {
				return 0, nil, false
			}
			params = append(params, b[off:off+l])
			off += l
		}
	}

	return nalLenSize, params, true
}

// parseStsz reads the sample size table. The sample count is a 32-bit value
// taken directly from the file, so allocations derived from it are bounded:
// per-entry sizes must be present in the box payload, and a uniform table
// still implies one byte of media data per sample, so its count cannot
// exceed the input size. Without these caps, a tiny forged stsz box forces
// a multi-gigabyte allocation.
func parseStsz(p []byte, maxSamples int) []int64 {
	if len(p) < 12 {
		return nil
	}

	uniform := binary.BigEndian.Uint32(p[4:8])
	n := int(binary.BigEndian.Uint32(p[8:12]))

	if uniform != 0 {
		if n > maxSamples {
			n = maxSamples
		}
	} else if n > (len(p)-12)/4 {
		n = (len(p) - 12) / 4
	}

	out := make([]int64, n)
	if uniform != 0 {
		for i := range out {
			out[i] = int64(uniform)
		}
		return out
	}

	off := 12
	for i := 0; i < n; i++ {
		if off+4 > len(p) {
			return out[:i]
		}
		out[i] = int64(binary.BigEndian.Uint32(p[off : off+4]))
		off += 4
	}

	return out
}

func parseOffsets(p []byte, sz int) []int64 {
	if len(p) < 8 {
		return nil
	}

	n := int(binary.BigEndian.Uint32(p[4:8]))
	if n > (len(p)-8)/sz {
		// Chunk offsets must be present in the box payload.
		n = (len(p) - 8) / sz
	}

	out := make([]int64, 0, n)
	off := 8
	for i := 0; i < n; i++ {
		if off+sz > len(p) {
			break
		}
		if sz == 8 {
			out = append(out, int64(binary.BigEndian.Uint64(p[off:off+8])))
		} else {
			out = append(out, int64(binary.BigEndian.Uint32(p[off:off+4])))
		}
		off += sz
	}

	return out
}

// parseStsc returns the sample-to-chunk table as flat [first_chunk, samples_per_chunk] pairs.
func parseStsc(p []byte) []int {
	if len(p) < 8 {
		return nil
	}

	n := int(binary.BigEndian.Uint32(p[4:8]))
	if n > (len(p)-8)/12 {
		// Entries must be present in the box payload.
		n = (len(p) - 8) / 12
	}

	out := make([]int, 0, n*2)
	off := 8
	for i := 0; i < n; i++ {
		if off+12 > len(p) {
			break
		}
		first := int(binary.BigEndian.Uint32(p[off : off+4]))
		spc := int(binary.BigEndian.Uint32(p[off+4 : off+8]))
		out = append(out, first, spc)
		off += 12
	}

	return out
}

// parseStts reads the sample-to-decode-time deltas. The entry count is
// bounded by the box payload and the expanded total (sum of per-entry
// counts) by the input size, so a forged entry count cannot expand into a
// huge slice.
func parseStts(p []byte, maxSamples int) []uint32 {
	if len(p) < 8 {
		return nil
	}

	n := int(binary.BigEndian.Uint32(p[4:8]))
	if n > (len(p)-8)/8 {
		n = (len(p) - 8) / 8
	}

	var out []uint32
	off := 8
	for i := 0; i < n; i++ {
		if off+8 > len(p) {
			break
		}
		cnt := binary.BigEndian.Uint32(p[off : off+4])
		delta := binary.BigEndian.Uint32(p[off+4 : off+8])
		for c := uint32(0); c < cnt && len(out) < maxSamples; c++ {
			out = append(out, delta)
		}
		off += 8
	}

	return out
}

// sampleOffsets computes each sample's absolute offset and size from the stsc/stco/stsz tables.
func sampleOffsets(chunks []int64, stsc []int, sizes []int64) []seqSample {
	nChunks := len(chunks)
	perChunk := make([]int, nChunks)

	for i := 0; i+1 < len(stsc); i += 2 {
		first := stsc[i] - 1
		spc := stsc[i+1]
		last := nChunks
		if i+2 < len(stsc) {
			last = stsc[i+2] - 1
		}
		for c := first; c < last && c >= 0 && c < nChunks; c++ {
			perChunk[c] = spc
		}
	}

	samples := make([]seqSample, 0, len(sizes))
	si := 0
	for c := 0; c < nChunks; c++ {
		off := chunks[c]
		for k := 0; k < perChunk[c] && si < len(sizes); k++ {
			samples = append(samples, seqSample{offset: off, size: sizes[si]})
			off += sizes[si]
			si++
		}
	}

	return samples
}

// annexB assembles a start-code Annex-B stream: parameter sets followed by each sample's NAL units.
func (info *seqInfo) annexB(data []byte) []byte {
	start := []byte{0, 0, 0, 1}
	var out []byte

	for _, p := range info.params {
		out = append(out, start...)
		out = append(out, p...)
	}

	for _, s := range info.samples {
		if s.offset < 0 || s.offset+s.size > int64(len(data)) {
			continue
		}
		sample := data[s.offset : s.offset+s.size]

		o := 0
		for o+info.nalLenSize <= len(sample) {
			l := 0
			for i := 0; i < info.nalLenSize; i++ {
				l = l<<8 | int(sample[o+i])
			}
			o += info.nalLenSize
			if l <= 0 || o+l > len(sample) {
				break
			}
			out = append(out, start...)
			out = append(out, sample[o:o+l]...)
			o += l
		}
	}

	return out
}
