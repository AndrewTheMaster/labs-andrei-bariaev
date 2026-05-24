package ir

// bitsNeededU32 — число бит для кодирования значений 0..max включительно.
func bitsNeededU32(max uint32) uint8 {
	if max == 0 {
		return 0
	}
	var n uint8
	for v := max; v > 0; v >>= 1 {
		n++
	}
	return n
}

type bitWriter struct {
	buf []byte
	acc uint64
	n   uint8
}

func (w *bitWriter) writeBits(v uint32, width uint8) {
	if width == 0 {
		return
	}
	for i := int(width) - 1; i >= 0; i-- {
		bit := (v >> uint(i)) & 1
		w.acc = (w.acc << 1) | uint64(bit)
		w.n++
		if w.n == 8 {
			w.buf = append(w.buf, byte(w.acc))
			w.acc = 0
			w.n = 0
		}
	}
}

func (w *bitWriter) finish() []byte {
	if w.n > 0 {
		w.acc <<= 8 - w.n
		w.buf = append(w.buf, byte(w.acc))
		w.acc = 0
		w.n = 0
	}
	return w.buf
}

type bitReader struct {
	data []byte
	i    int
	acc  uint64
	n    uint8
}

func (r *bitReader) readBits(width uint8) (uint32, error) {
	if width == 0 {
		return 0, nil
	}
	var v uint32
	for k := int(width) - 1; k >= 0; k-- {
		if r.n == 0 {
			if r.i >= len(r.data) {
				return 0, errBitpackTrunc
			}
			r.acc = uint64(r.data[r.i])
			r.i++
			r.n = 8
		}
		bit := (r.acc >> 7) & 1
		r.acc <<= 1
		r.n--
		v |= uint32(bit) << uint(k)
	}
	return v, nil
}

func packUint32Stream(vals []uint32) (width uint8, payload []byte) {
	var max uint32
	for _, v := range vals {
		if v > max {
			max = v
		}
	}
	width = bitsNeededU32(max)
	var w bitWriter
	for _, v := range vals {
		w.writeBits(v, width)
	}
	return width, w.finish()
}

func unpackUint32Stream(width uint8, payload []byte, count int) ([]uint32, error) {
	out := make([]uint32, count)
	if count == 0 {
		return out, nil
	}
	r := bitReader{data: payload}
	for i := 0; i < count; i++ {
		v, err := r.readBits(width)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}
