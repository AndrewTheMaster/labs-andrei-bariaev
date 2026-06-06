package ir

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

func putUvarint(buf *bytes.Buffer, v uint64) {
	var b [10]byte
	n := binary.PutUvarint(b[:], v)
	buf.Write(b[:n])
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
