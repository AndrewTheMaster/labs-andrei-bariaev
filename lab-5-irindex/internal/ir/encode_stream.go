package ir

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	streamVarint  = 0
	streamBitpack = 1
)

func putUvarint(buf *bytes.Buffer, v uint64) {
	var b [10]byte
	n := binary.PutUvarint(b[:], v)
	buf.Write(b[:n])
}

func encodeVarintStream(vals []uint32) []byte {
	var buf bytes.Buffer
	for _, v := range vals {
		putUvarint(&buf, uint64(v))
	}
	return buf.Bytes()
}

func decodeVarintStream(payload []byte, count int) ([]uint32, error) {
	out := make([]uint32, count)
	i := 0
	for k := 0; k < count; k++ {
		v, n := binary.Uvarint(payload[i:])
		if n <= 0 {
			return nil, fmt.Errorf("varint stream truncated at %d", k)
		}
		out[k] = uint32(v)
		i += n
	}
	return out, nil
}

// encodeOptimalStream — varint или bitpack, что компактнее (tf, pos Δ).
func encodeOptimalStream(vals []uint32) (codec byte, payload []byte) {
	if len(vals) == 0 {
		return streamVarint, nil
	}
	vi := encodeVarintStream(vals)
	width, bp := packUint32Stream(vals)
	bitpackPayload := append([]byte{width}, bp...)
	if len(vi) <= len(bitpackPayload) {
		return streamVarint, vi
	}
	return streamBitpack, bitpackPayload
}

func decodeOptimalStream(codec byte, payload []byte, count int) ([]uint32, error) {
	if count == 0 {
		return nil, nil
	}
	switch codec {
	case streamVarint:
		return decodeVarintStream(payload, count)
	case streamBitpack:
		if len(payload) < 1 {
			return nil, fmt.Errorf("bitpack stream empty")
		}
		return unpackUint32Stream(payload[0], payload[1:], count)
	default:
		return nil, fmt.Errorf("unknown stream codec %d", codec)
	}
}
