package heic

import (
	"encoding/binary"
	"io"
)

// exifPayload streams the top-level boxes, keeping only meta, then reaches the Exif item via its iloc extent.
func exifPayload(r io.Reader) []byte {
	var pos int64
	var hdr [8]byte

	for {
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			return nil
		}
		pos += 8

		size := int64(binary.BigEndian.Uint32(hdr[0:4]))
		typ := string(hdr[4:8])

		body := size - 8
		if size == 1 {
			var big [8]byte
			if _, err := io.ReadFull(r, big[:]); err != nil {
				return nil
			}
			pos += 8
			body = int64(binary.BigEndian.Uint64(big[:])) - 16
		} else if size == 0 {
			body = -1
		}

		if typ == "meta" {
			meta := readBody(r, body)
			if len(meta) < 4 {
				return nil
			}
			pos += int64(len(meta))

			return exifFromMeta(r, meta[4:], pos)
		}

		if body < 0 {
			return nil
		}
		if _, err := io.CopyN(io.Discard, r, body); err != nil {
			return nil
		}
		pos += body
	}
}

// exifFromMeta resolves the Exif item from the meta children and reads its TIFF payload from r at absolute pos.
func exifFromMeta(r io.Reader, meta []byte, pos int64) []byte {
	id := exifItemID(meta)
	if id < 0 {
		return nil
	}

	off, length, method, ok := ilocExtent(meta, id)
	if !ok || length < 4 {
		return nil
	}

	var raw []byte
	switch method {
	case 0:
		if int64(off) < pos {
			return nil // Exif precedes the meta box; not reachable by forward streaming.
		}
		if _, err := io.CopyN(io.Discard, r, int64(off)-pos); err != nil {
			return nil
		}
		raw = readBody(r, int64(length))
	case 1:
		idat := idatPayload(meta)
		if idat == nil || off+length > uint64(len(idat)) {
			return nil
		}
		raw = idat[off : off+length]
	default:
		return nil
	}

	if len(raw) < 4 {
		return nil
	}

	start := 4 + int(binary.BigEndian.Uint32(raw[0:4]))
	if start >= len(raw) {
		return nil
	}

	return raw[start:]
}

// readBody reads n bytes, or all remaining bytes when n is negative.
func readBody(r io.Reader, n int64) []byte {
	if n < 0 {
		b, _ := io.ReadAll(r)
		return b
	}

	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil
	}

	return b
}

// eachBox iterates the child boxes within b, invoking fn(type, payload) until fn returns false.
func eachBox(b []byte, fn func(typ string, payload []byte) bool) {
	off := 0
	for off+8 <= len(b) {
		size := int(binary.BigEndian.Uint32(b[off : off+4]))
		typ := string(b[off+4 : off+8])
		hdr := 8

		if size == 1 {
			if off+16 > len(b) {
				return
			}
			size = int(binary.BigEndian.Uint64(b[off+8 : off+16]))
			hdr = 16
		} else if size == 0 {
			size = len(b) - off
		}

		if size < hdr || off+size > len(b) {
			return
		}

		if !fn(typ, b[off+hdr:off+size]) {
			return
		}

		off += size
	}
}

// exifItemID returns the item ID of the Exif item from the iinf box, or -1 when absent.
func exifItemID(meta []byte) int {
	id := -1

	eachBox(meta, func(typ string, payload []byte) bool {
		if typ != "iinf" {
			return true
		}

		start := 6
		if payload[0] != 0 {
			start = 8
		}
		if start > len(payload) {
			return false
		}

		eachBox(payload[start:], func(t string, p []byte) bool {
			if t != "infe" || len(p) < 1 {
				return true
			}

			var itemID int
			var itemType string
			if p[0] == 2 && len(p) >= 12 {
				itemID = int(binary.BigEndian.Uint16(p[4:6]))
				itemType = string(p[8:12])
			} else if p[0] >= 3 && len(p) >= 14 {
				itemID = int(binary.BigEndian.Uint32(p[4:8]))
				itemType = string(p[10:14])
			} else {
				return true
			}

			if itemType == "Exif" {
				id = itemID
				return false
			}

			return true
		})

		return false
	})

	return id
}

// ilocExtent returns the first extent's absolute offset, length and construction method for item.
func ilocExtent(meta []byte, item int) (offset, length uint64, method int, ok bool) {
	eachBox(meta, func(typ string, p []byte) bool {
		if typ != "iloc" || len(p) < 8 {
			return true
		}

		version := p[0]
		offsetSize := int(p[4] >> 4)
		lengthSize := int(p[4] & 0xf)
		baseOffsetSize := int(p[5] >> 4)
		indexSize := int(p[5] & 0xf)

		off := 8
		var itemCount int
		if version < 2 {
			itemCount = int(binary.BigEndian.Uint16(p[6:8]))
		} else {
			if len(p) < 10 {
				return false
			}
			itemCount = int(binary.BigEndian.Uint32(p[6:10]))
			off = 10
		}

		for i := 0; i < itemCount; i++ {
			var id int
			if version < 2 {
				if off+2 > len(p) {
					return false
				}
				id = int(binary.BigEndian.Uint16(p[off : off+2]))
				off += 2
			} else {
				if off+4 > len(p) {
					return false
				}
				id = int(binary.BigEndian.Uint32(p[off : off+4]))
				off += 4
			}

			m := 0
			if version == 1 || version == 2 {
				if off+2 > len(p) {
					return false
				}
				m = int(binary.BigEndian.Uint16(p[off:off+2]) & 0xf)
				off += 2
			}

			off += 2 // data_reference_index

			base, good := readUint(p, &off, baseOffsetSize)
			if !good {
				return false
			}

			if off+2 > len(p) {
				return false
			}
			extents := int(binary.BigEndian.Uint16(p[off : off+2]))
			off += 2

			var eo, el uint64
			for e := 0; e < extents; e++ {
				if (version == 1 || version == 2) && indexSize > 0 {
					if _, good = readUint(p, &off, indexSize); !good {
						return false
					}
				}

				o, ok1 := readUint(p, &off, offsetSize)
				l, ok2 := readUint(p, &off, lengthSize)
				if !ok1 || !ok2 {
					return false
				}
				if e == 0 {
					eo, el = o, l
				}
			}

			if id == item {
				offset, length, method, ok = base+eo, el, m, true
				return false
			}
		}

		return false
	})

	return offset, length, method, ok
}

// idatPayload returns the idat box content of the meta box, or nil when absent.
func idatPayload(meta []byte) []byte {
	var out []byte

	eachBox(meta, func(typ string, p []byte) bool {
		if typ == "idat" {
			out = p
			return false
		}
		return true
	})

	return out
}

// readUint reads a big-endian integer of size bytes at off and advances off.
func readUint(b []byte, off *int, size int) (uint64, bool) {
	if size == 0 {
		return 0, true
	}
	if *off+size > len(b) {
		return 0, false
	}

	var v uint64
	for i := 0; i < size; i++ {
		v = v<<8 | uint64(b[*off+i])
	}
	*off += size

	return v, true
}
