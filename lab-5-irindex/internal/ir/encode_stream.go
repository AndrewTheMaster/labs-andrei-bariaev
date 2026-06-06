package ir

import (
	"bytes"
	"fmt"
)

const (
	streamVarint  = 0
	streamBitpack = 1
)

func encodeVarintStream(vals []uint32) []byte {
	var buf bytes.Buffer
	for _, v := range vals {
		putUvarint(&buf, uint64(v))
	}
	return buf.Bytes()
}

// encodeBitpackStream — tf/pos Δ всегда bitpack (ширина + payload).
func encodeBitpackStream(vals []uint32) (codec byte, payload []byte) {
	if len(vals) == 0 {
		return streamBitpack, nil
	}
	width, bp := packUint32Stream(vals)
	return streamBitpack, append([]byte{width}, bp...)
}

// encodeOptimalStream — varint или bitpack для tf/pos Δ (что компактнее).
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

func decodeBitpackStream(payload []byte, count int) ([]uint32, error) {
	if count == 0 {
		return nil, nil
	}
	if len(payload) < 1 {
		return nil, fmt.Errorf("bitpack stream empty")
	}
	return unpackUint32Stream(payload[0], payload[1:], count)
}

func decodeStream(codec byte, payload []byte, count int) ([]uint32, error) {
	if count == 0 {
		return nil, nil
	}
	switch codec {
	case streamVarint:
		return decodeVarintStream(payload, count)
	case streamBitpack:
		return decodeBitpackStream(payload, count)
	default:
		return nil, fmt.Errorf("unknown stream codec %d", codec)
	}
}
