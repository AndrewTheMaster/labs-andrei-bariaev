package ir

import (
	"encoding/binary"
	"fmt"
)

const p4BlockSize = 128
const p4MaxExceptions = 32

// encodePForDelta — PForDelta по блокам (doc Δ и другие монотонные/малые потоки).
func encodePForDelta(vals []uint32) []byte {
	if len(vals) == 0 {
		return nil
	}
	var out []byte
	var cnt [4]byte
	binary.LittleEndian.PutUint32(cnt[:], uint32(len(vals)))
	out = append(out, cnt[:]...)
	for off := 0; off < len(vals); {
		end := off + p4BlockSize
		if end > len(vals) {
			end = len(vals)
		}
		block := vals[off:end]
		out = append(out, encodeP4Block(block)...)
		off = end
	}
	return out
}

func encodeP4Block(vals []uint32) []byte {
	n := len(vals)
	if n == 0 {
		return nil
	}
	bestB, bestExc, bestExcVals := chooseP4Params(vals)
	out := make([]byte, 0, 4+(n*int(bestB)+7)/8+len(bestExc)*(1+4))
	out = append(out, byte(n), bestB, byte(len(bestExc)))
	out = append(out, packBits(vals, bestB, bestExc)...)
	for _, pos := range bestExc {
		out = append(out, byte(pos))
	}
	for _, v := range bestExcVals {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], v)
		out = append(out, b[:]...)
	}
	return out
}

func chooseP4Params(vals []uint32) (uint8, []int, []uint32) {
	n := len(vals)
	bestSize := int(^uint(0) >> 1)
	var bestB uint8
	var bestExc []int
	var bestExcVals []uint32
	for b := 0; b <= 32; b++ {
		var limit uint32
		if b == 32 {
			limit = ^uint32(0)
		} else {
			limit = (1 << uint(b)) - 1
		}
		var excPos []int
		var excVals []uint32
		for i, v := range vals {
			if v > limit {
				excPos = append(excPos, i)
				excVals = append(excVals, v)
			}
		}
		if len(excPos) > p4MaxExceptions {
			continue
		}
		sz := 3 + (n*b+7)/8 + len(excPos)*5
		if sz < bestSize {
			bestSize = sz
			bestB = uint8(b)
			bestExc = excPos
			bestExcVals = excVals
		}
	}
	if bestSize == int(^uint(0)>>1) {
		// fallback: все как exceptions при b=0
		bestB = 0
		for i, v := range vals {
			bestExc = append(bestExc, i)
			bestExcVals = append(bestExcVals, v)
		}
	}
	return bestB, bestExc, bestExcVals
}

func packBits(vals []uint32, width uint8, skipPos []int) []byte {
	skip := make(map[int]struct{}, len(skipPos))
	for _, p := range skipPos {
		skip[p] = struct{}{}
	}
	var w bitWriter
	for i, v := range vals {
		if _, ok := skip[i]; ok {
			v = 0
		}
		w.writeBits(v, width)
	}
	return w.finish()
}

func decodePForDelta(data []byte, count int) ([]uint32, error) {
	if count == 0 {
		return nil, nil
	}
	if len(data) < 4 {
		return nil, fmt.Errorf("p4: truncated header")
	}
	n := int(binary.LittleEndian.Uint32(data[0:4]))
	if n != count {
		return nil, fmt.Errorf("p4: count mismatch %d vs %d", n, count)
	}
	out := make([]uint32, 0, n)
	i := 4
	for len(out) < n {
		if i+3 > len(data) {
			return nil, fmt.Errorf("p4: truncated block header")
		}
		blen := int(data[i])
		bits := data[i+1]
		nExc := int(data[i+2])
		i += 3
		if blen == 0 || blen > p4BlockSize {
			return nil, fmt.Errorf("p4: bad block len %d", blen)
		}
		packLen := (blen*int(bits) + 7) / 8
		if i+packLen+nExc*5 > len(data) {
			return nil, fmt.Errorf("p4: truncated block body")
		}
		payload := data[i : i+packLen]
		i += packLen
		excPos := make([]int, nExc)
		excVals := make([]uint32, nExc)
		for j := 0; j < nExc; j++ {
			excPos[j] = int(data[i])
			i++
		}
		for j := 0; j < nExc; j++ {
			excVals[j] = binary.LittleEndian.Uint32(data[i : i+4])
			i += 4
		}
		block, err := unpackP4Block(payload, blen, bits, excPos, excVals)
		if err != nil {
			return nil, err
		}
		out = append(out, block...)
	}
	return out, nil
}

func unpackP4Block(payload []byte, n int, width uint8, excPos []int, excVals []uint32) ([]uint32, error) {
	out := make([]uint32, n)
	r := bitReader{data: payload}
	excMap := make(map[int]uint32, len(excPos))
	for j, p := range excPos {
		excMap[p] = excVals[j]
	}
	for k := 0; k < n; k++ {
		v, err := r.readBits(width)
		if err != nil {
			return nil, err
		}
		if ev, ok := excMap[k]; ok {
			v = ev
		}
		out[k] = v
	}
	return out, nil
}
