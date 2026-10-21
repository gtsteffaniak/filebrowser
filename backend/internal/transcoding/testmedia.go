package transcoding

import "github.com/gtsteffaniak/go-ffmpeg/mp4"

// fakeVODSegmentPayload is a minimal fMP4 fragment for tests and FakeEngine.
func fakeVODSegmentPayload() []byte {
	return buildTestFragmentWithTFDTForTrack(0, 1, mp4.MinHLSSegmentMediaBytes)
}

func buildTestFragmentWithTFDTForTrack(ticks uint32, trackID uint32, minTotal int) []byte {
	tfdt := makeFullAtom("tfdt", 0, uint32ToBytes(ticks))
	tfhd := makeFullAtom("tfhd", 0, uint32ToBytes(trackID))
	// A trun keeps the moof above the reader's minimum parseable size.
	trunPayload := append([]byte{0, 0x00, 0x01, 0x00}, uint32ToBytes(2)...)
	trunPayload = append(trunPayload, uint32ToBytes(1024)...)
	trunPayload = append(trunPayload, uint32ToBytes(1024)...)
	trun := makeAtom("trun", trunPayload)
	trafPayload := append(tfhd, tfdt...)
	trafPayload = append(trafPayload, trun...)
	traf := makeAtom("traf", trafPayload)
	moof := makeAtom("moof", traf)
	pad := minTotal - len(moof) - 8
	if pad < 1 {
		pad = 1
	}
	mdat := makeAtom("mdat", make([]byte, pad))
	return append(moof, mdat...)
}

func makeAtom(typ string, payload []byte) []byte {
	out := make([]byte, 8+len(payload))
	putU32(out[0:4], uint32(8+len(payload)))
	copy(out[4:8], typ)
	copy(out[8:], payload)
	return out
}

func makeFullAtom(typ string, version byte, payload []byte) []byte {
	body := append([]byte{version, 0, 0, 0}, payload...)
	return makeAtom(typ, body)
}

func putU32(b []byte, v uint32) {
	b[0] = byte(v >> 24)
	b[1] = byte(v >> 16)
	b[2] = byte(v >> 8)
	b[3] = byte(v)
}

func uint32ToBytes(v uint32) []byte {
	return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}
