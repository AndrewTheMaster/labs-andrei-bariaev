package ir

import "bytes"

func docDeltas(ps []posting) []uint32 {
	docVals := make([]uint32, len(ps))
	var prevDoc uint32
	for i, p := range ps {
		if i == 0 {
			docVals[i] = p.DocID
		} else {
			docVals[i] = p.DocID - prevDoc
		}
		prevDoc = p.DocID
	}
	return docVals
}

func docStreamBytesP4(ps []posting) int {
	return len(encodePForDelta(docDeltas(ps)))
}

func docStreamBytesBitpack(ps []posting) int {
	vals := docDeltas(ps)
	if len(vals) == 0 {
		return 0
	}
	_, pay := packUint32Stream(vals)
	return 1 + len(pay)
}

// AnalyzeDocCodecBreakdown — сравнение только doc Δ: PForDelta vs bitpack.
func AnalyzeDocCodecBreakdown(ix *InvIndex) (p4Total, bpTotal int64, p4wins, bpwins, tie int) {
	for _, ps := range ix.postings {
		p4 := int64(docStreamBytesP4(ps))
		bp := int64(docStreamBytesBitpack(ps))
		p4Total += p4
		bpTotal += bp
		switch {
		case p4 < bp:
			p4wins++
		case p4 > bp:
			bpwins++
		default:
			tie++
		}
	}
	return
}

func putUvarintLegacy(buf *bytes.Buffer, v uint64) {
	putUvarint(buf, v)
}

// encodePostingsVarint — IRIXV1 baseline (delta + uvarint по всем потокам).
func encodePostingsVarint(ps []posting) []byte {
	var buf bytes.Buffer
	putUvarintLegacy(&buf, uint64(len(ps)))
	var prevDoc uint32
	for i, p := range ps {
		if i == 0 {
			putUvarintLegacy(&buf, uint64(p.DocID))
		} else {
			putUvarintLegacy(&buf, uint64(p.DocID-prevDoc))
		}
		prevDoc = p.DocID
		putUvarintLegacy(&buf, uint64(len(p.Poss)))
		var prevPos uint32
		for j, pos := range p.Poss {
			if j == 0 {
				putUvarintLegacy(&buf, uint64(pos))
			} else {
				putUvarintLegacy(&buf, uint64(pos-prevPos))
			}
			prevPos = pos
		}
	}
	return buf.Bytes()
}

// encodePostingsBitpackAll — IRIXV2 baseline (bitpack doc/tf/pos Δ).
func encodePostingsBitpackAll(ps []posting) []byte {
	var buf bytes.Buffer
	writeU32(&buf, uint32(len(ps)))
	if len(ps) == 0 {
		return buf.Bytes()
	}
	docVals := make([]uint32, len(ps))
	tfVals := make([]uint32, len(ps))
	var posVals []uint32
	var prevDoc uint32
	for i, p := range ps {
		if i == 0 {
			docVals[i] = p.DocID
		} else {
			docVals[i] = p.DocID - prevDoc
		}
		prevDoc = p.DocID
		tfVals[i] = uint32(len(p.Poss))
		var prevPos uint32
		for j, pos := range p.Poss {
			if j == 0 {
				posVals = append(posVals, pos)
			} else {
				posVals = append(posVals, pos-prevPos)
			}
			prevPos = pos
		}
	}
	docBits, docPay := packUint32Stream(docVals)
	tfBits, tfPay := packUint32Stream(tfVals)
	posBits, posPay := packUint32Stream(posVals)
	buf.WriteByte(docBits)
	buf.WriteByte(tfBits)
	buf.WriteByte(posBits)
	buf.WriteByte(0)
	writeChunk(&buf, docPay)
	writeChunk(&buf, tfPay)
	writeChunk(&buf, posPay)
	return buf.Bytes()
}
