package codec

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// ---------------------------------------------------------------------------
// Encode
// ---------------------------------------------------------------------------

func TestEncodeBasicRoundTrip(t *testing.T) {
	seq := uint64(12345)
	ts := int64(9876543210)
	lastSent := uint32(500)
	srcPort := uint16(43500)
	dstPort := uint16(43505)
	salt := bytes.Repeat([]byte{0xFF}, 64)

	data := Encode(seq, salt, ts, MsgHeaderLen+len(salt), lastSent, srcPort, dstPort)

	if len(data) != MsgHeaderLen+len(salt) {
		t.Fatalf("len = %d, want %d", len(data), MsgHeaderLen+len(salt))
	}
	if !IsValid(data) {
		t.Fatal("encoded data should be valid")
	}

	gotSeq, gotTS, gotLastSent, gotSrcPort, gotDstPort := Decode(data)
	if gotSeq != seq {
		t.Errorf("seq = %d, want %d", gotSeq, seq)
	}
	if gotTS != ts {
		t.Errorf("ts = %d, want %d", gotTS, ts)
	}
	if gotLastSent != lastSent {
		t.Errorf("lastSent = %d, want %d", gotLastSent, lastSent)
	}
	if gotSrcPort != srcPort {
		t.Errorf("srcPort = %d, want %d", gotSrcPort, srcPort)
	}
	if gotDstPort != dstPort {
		t.Errorf("dstPort = %d, want %d", gotDstPort, dstPort)
	}
	if !bytes.Equal(data[MsgHeaderLen:], salt) {
		t.Error("salt payload mismatch")
	}
}

func TestEncodeExactHeaderLen(t *testing.T) {
	// msgLen == MsgHeaderLen → no salt room
	data := Encode(42, nil, 100, MsgHeaderLen, 200, 0, 0)
	if len(data) != MsgHeaderLen {
		t.Fatalf("len = %d, want %d", len(data), MsgHeaderLen)
	}
	if !IsValid(data) {
		t.Error("header-only packet should be valid")
	}
}

func TestEncodeBelowMinimumLen(t *testing.T) {
	// msgLen < 32 should be clamped to 32
	for _, msgLen := range []int{0, 1, 10, 20, 27, 31} {
		data := Encode(1, nil, 2, msgLen, 3, 0, 0)
		if len(data) != MsgHeaderLen {
			t.Errorf("msgLen=%d: len=%d, want %d", msgLen, len(data), MsgHeaderLen)
		}
	}
}

func TestEncodeMagicPlacement(t *testing.T) {
	data := Encode(0, nil, 0, MsgHeaderLen, 0, 0, 0)
	if !bytes.Equal(data[:magicFlagLen], magicFlag) {
		t.Errorf("magic = %x, want %x", data[:magicFlagLen], magicFlag)
	}
}

func TestEncodeSeqPlacement(t *testing.T) {
	seq := uint64(0xDEADBEEFCAFE1234)
	data := Encode(seq, nil, 0, MsgHeaderLen, 0, 0, 0)
	got := binary.BigEndian.Uint64(data[magicFlagLen : magicFlagLen+8])
	if got != seq {
		t.Errorf("seq = %x, want %x", got, seq)
	}
}

func TestEncodeTsPlacement(t *testing.T) {
	ts := int64(-1234567890123456789)
	data := Encode(0, nil, ts, MsgHeaderLen, 0, 0, 0)
	got := int64(binary.BigEndian.Uint64(data[magicFlagLen+8 : magicFlagLen+16]))
	if got != ts {
		t.Errorf("ts = %d, want %d", got, ts)
	}
}

func TestEncodeLastSentPlacement(t *testing.T) {
	lastSent := uint32(0xAABBCCDD)
	data := Encode(0, nil, 0, MsgHeaderLen, lastSent, 0, 0)
	got := binary.BigEndian.Uint32(data[magicFlagLen+16 : magicFlagLen+20])
	if got != lastSent {
		t.Errorf("lastSent = %x, want %x", got, lastSent)
	}
}

func TestEncodeStartPortsPlacement(t *testing.T) {
	srcPort := uint16(0xAABB)
	dstPort := uint16(0xCCDD)
	data := Encode(0, nil, 0, MsgHeaderLen, 0, srcPort, dstPort)
	gotSrc := binary.BigEndian.Uint16(data[magicFlagLen+20 : magicFlagLen+22])
	gotDst := binary.BigEndian.Uint16(data[magicFlagLen+22 : magicFlagLen+24])
	if gotSrc != srcPort {
		t.Errorf("lastStartSrcPort = %x, want %x", gotSrc, srcPort)
	}
	if gotDst != dstPort {
		t.Errorf("lastStartDstPort = %x, want %x", gotDst, dstPort)
	}
}

func TestEncodeSaltTruncation(t *testing.T) {
	// Salt longer than available space should be truncated by copy
	msgLen := MsgHeaderLen + 4
	longSalt := bytes.Repeat([]byte{0xAB}, 100)
	data := Encode(1, longSalt, 2, msgLen, 3, 0, 0)
	if len(data) != msgLen {
		t.Fatalf("len = %d, want %d", len(data), msgLen)
	}
	// Only first 4 bytes of salt should be copied
	if !bytes.Equal(data[MsgHeaderLen:], []byte{0xAB, 0xAB, 0xAB, 0xAB}) {
		t.Errorf("salt not truncated correctly: %x", data[MsgHeaderLen:])
	}
}

func TestEncodeZeroValues(t *testing.T) {
	data := Encode(0, nil, 0, MsgHeaderLen, 0, 0, 0)
	if !IsValid(data) {
		t.Error("zero-value packet should be valid")
	}
	seq, ts, lastSent, srcPort, dstPort := Decode(data)
	if seq != 0 || ts != 0 || lastSent != 0 || srcPort != 0 || dstPort != 0 {
		t.Errorf("zero-value decode: seq=%d ts=%d lastSent=%d srcPort=%d dstPort=%d", seq, ts, lastSent, srcPort, dstPort)
	}
}

func TestEncodeMaxValues(t *testing.T) {
	seq := uint64(1<<64 - 1)
	ts := int64(1<<63 - 1)
	lastSent := uint32(1<<32 - 1)
	srcPort := uint16(1<<16 - 1)
	dstPort := uint16(1<<16 - 1)
	data := Encode(seq, bytes.Repeat([]byte{0x00}, 10), ts, MsgHeaderLen+10, lastSent, srcPort, dstPort)

	gotSeq, gotTS, gotLastSent, gotSrcPort, gotDstPort := Decode(data)
	if gotSeq != seq {
		t.Errorf("seq = %d, want %d", gotSeq, seq)
	}
	if gotTS != ts {
		t.Errorf("ts = %d, want %d", gotTS, ts)
	}
	if gotLastSent != lastSent {
		t.Errorf("lastSent = %d, want %d", gotLastSent, lastSent)
	}
	if gotSrcPort != srcPort {
		t.Errorf("srcPort = %d, want %d", gotSrcPort, srcPort)
	}
	if gotDstPort != dstPort {
		t.Errorf("dstPort = %d, want %d", gotDstPort, dstPort)
	}
}

func TestEncodeNegativeTs(t *testing.T) {
	ts := int64(-9999999999)
	data := Encode(1, nil, ts, MsgHeaderLen, 0, 0, 0)
	gotSeq, gotTS, _, _, _ := Decode(data)
	if gotSeq != 1 {
		t.Errorf("seq = %d, want 1", gotSeq)
	}
	if gotTS != ts {
		t.Errorf("ts = %d, want %d", gotTS, ts)
	}
}

func TestEncodeLargePayload(t *testing.T) {
	msgLen := 1400
	salt := bytes.Repeat([]byte{0x5A}, msgLen-MsgHeaderLen)
	data := Encode(100, salt, 200, msgLen, 300, 0, 0)
	if len(data) != msgLen {
		t.Fatalf("len = %d, want %d", len(data), msgLen)
	}
	if !bytes.Equal(data[MsgHeaderLen:], salt) {
		t.Error("large salt payload mismatch")
	}
}

func TestEncodeMultipleSalts(t *testing.T) {
	for _, pat := range []struct {
		idx   int
		value byte
	}{
		{0, 0xFF},
		{1, 0x00},
		{2, 0x5A},
	} {
		seq := uint64(pat.idx)
		salt := bytes.Repeat([]byte{pat.value}, 64)
		data := Encode(seq, salt, 0, MsgHeaderLen+64, 0, 0, 0)
		if !bytes.Equal(data[MsgHeaderLen:], salt) {
			t.Errorf("salt pattern %d mismatch", pat.idx)
		}
	}
}

// ---------------------------------------------------------------------------
// Decode
// ---------------------------------------------------------------------------

func TestDecodeValidPacket(t *testing.T) {
	seq := uint64(777)
	ts := int64(123456789)
	lastSent := uint32(42)
	srcPort := uint16(43500)
	dstPort := uint16(43505)
	data := Encode(seq, nil, ts, MsgHeaderLen, lastSent, srcPort, dstPort)

	gotSeq, gotTS, gotLastSent, gotSrcPort, gotDstPort := Decode(data)
	if gotSeq != seq {
		t.Errorf("seq = %d, want %d", gotSeq, seq)
	}
	if gotTS != ts {
		t.Errorf("ts = %d, want %d", gotTS, ts)
	}
	if gotLastSent != lastSent {
		t.Errorf("lastSent = %d, want %d", gotLastSent, lastSent)
	}
	if gotSrcPort != srcPort {
		t.Errorf("srcPort = %d, want %d", gotSrcPort, srcPort)
	}
	if gotDstPort != dstPort {
		t.Errorf("dstPort = %d, want %d", gotDstPort, dstPort)
	}
}

func TestDecodeEmptySlice(t *testing.T) {
	seq, ts, lastSent, srcPort, dstPort := Decode(nil)
	if seq != 0 || ts != 0 || lastSent != 0 || srcPort != 0 || dstPort != 0 {
		t.Errorf("expected zero values for nil, got seq=%d ts=%d lastSent=%d srcPort=%d dstPort=%d", seq, ts, lastSent, srcPort, dstPort)
	}
}

func TestDecodeShortSlice(t *testing.T) {
	data := make([]byte, MsgHeaderLen-1)
	seq, ts, lastSent, srcPort, dstPort := Decode(data)
	if seq != 0 || ts != 0 || lastSent != 0 || srcPort != 0 || dstPort != 0 {
		t.Errorf("expected zero values for short input, got seq=%d ts=%d lastSent=%d srcPort=%d dstPort=%d", seq, ts, lastSent, srcPort, dstPort)
	}
}

func TestDecodeExactlyHeaderLen(t *testing.T) {
	data := Encode(42, nil, 100, MsgHeaderLen, 200, 300, 400)
	seq, ts, lastSent, srcPort, dstPort := Decode(data)
	if seq != 42 || ts != 100 || lastSent != 200 || srcPort != 300 || dstPort != 400 {
		t.Errorf("decode at exact header len: seq=%d ts=%d lastSent=%d srcPort=%d dstPort=%d", seq, ts, lastSent, srcPort, dstPort)
	}
}

func TestDecodeWithExtraPayload(t *testing.T) {
	data := Encode(10, bytes.Repeat([]byte{0xAA}, 50), 20, MsgHeaderLen+50, 30, 40, 50)
	seq, ts, lastSent, srcPort, dstPort := Decode(data)
	if seq != 10 || ts != 20 || lastSent != 30 || srcPort != 40 || dstPort != 50 {
		t.Errorf("decode with payload: seq=%d ts=%d lastSent=%d srcPort=%d dstPort=%d", seq, ts, lastSent, srcPort, dstPort)
	}
}

// ---------------------------------------------------------------------------
// IsValid
// ---------------------------------------------------------------------------

func TestIsValidTable(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"nil", nil, false},
		{"empty", []byte{}, false},
		{"too short", []byte("CHAO"), false},
		{"magic only 8 bytes", magicFlag, false},
		{"magic + 23 bytes (one short)", append([]byte("CHAOOAHC"), make([]byte, 23)...), false},
		{"exactly valid", Encode(1, nil, 2, MsgHeaderLen, 3, 0, 0), true},
		{"wrong magic", append([]byte("XXXXXXXX"), make([]byte, 24)...), false},
		{"partial magic", append([]byte("CHAOOAHX"), make([]byte, 24)...), false},
		{"valid with payload", Encode(1, bytes.Repeat([]byte{0xFF}, 100), 2, MsgHeaderLen+100, 3, 0, 0), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValid(tt.data); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidBoundaryLength(t *testing.T) {
	// 31 bytes: magic(8) + 23 bytes → one byte short
	short := make([]byte, 31)
	copy(short, magicFlag)
	if IsValid(short) {
		t.Error("31 bytes should be invalid")
	}

	// 32 bytes: magic(8) + 24 bytes → exactly valid
	exact := make([]byte, 32)
	copy(exact, magicFlag)
	if !IsValid(exact) {
		t.Error("32 bytes with correct magic should be valid")
	}
}

// ---------------------------------------------------------------------------
// ComplementaryBytes
// ---------------------------------------------------------------------------

func TestComplementaryBytesPattern(t *testing.T) {
	b := ComplementaryBytes(8)
	want := []byte{0xAA, 0xAA, 0x55, 0x55, 0xAA, 0xAA, 0x55, 0x55}
	if !bytes.Equal(b, want) {
		t.Errorf("got %x, want %x", b, want)
	}
}

func TestComplementaryBytesLength(t *testing.T) {
	for _, n := range []int{0, 1, 3, 10, 100, 4096} {
		got := ComplementaryBytes(n)
		if len(got) != n {
			t.Errorf("n=%d: len=%d, want %d", n, len(got), n)
		}
	}
}

func TestComplementaryBytesZeroLength(t *testing.T) {
	b := ComplementaryBytes(0)
	if len(b) != 0 {
		t.Errorf("expected empty slice for n=0, got len=%d", len(b))
	}
}

func TestComplementaryBytesOddLength(t *testing.T) {
	b := ComplementaryBytes(5)
	want := []byte{0xAA, 0xAA, 0x55, 0x55, 0xAA}
	if !bytes.Equal(b, want) {
		t.Errorf("got %x, want %x", b, want)
	}
}

func TestComplementaryBytesReproducible(t *testing.T) {
	a := ComplementaryBytes(1024)
	b := ComplementaryBytes(1024)
	if !bytes.Equal(a, b) {
		t.Error("complementary bytes should be identical across calls")
	}
}

// ---------------------------------------------------------------------------
// EncodeUDPPacket
// ---------------------------------------------------------------------------

func TestEncodeUDPPacketBasic(t *testing.T) {
	payload := []byte("hello")
	data, err := EncodeUDPPacket(
		[]byte{192, 168, 1, 1},
		[]byte{10, 0, 0, 2},
		12345, 80, 64, 64, payload,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty packet")
	}
}

func TestEncodeUDPPacketLength(t *testing.T) {
	payload := make([]byte, 100)
	data, err := EncodeUDPPacket(
		[]byte{192, 168, 1, 1},
		[]byte{10, 0, 0, 2},
		12345, 80, 64, 64, payload,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// UDP header (8) + payload (100) = 108
	expectedUDPLen := 8 + len(payload)
	if len(data) != expectedUDPLen {
		t.Errorf("packet len = %d, want %d (UDP header + payload)", len(data), expectedUDPLen)
	}
}

func TestEncodeUDPPacketPorts(t *testing.T) {
	srcPort := uint16(43500)
	dstPort := uint16(43509)
	payload := []byte("test")

	data, err := EncodeUDPPacket(
		[]byte{127, 0, 0, 1},
		[]byte{127, 0, 0, 1},
		srcPort, dstPort, 0, 64, payload,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotSrc := binary.BigEndian.Uint16(data[0:2])
	gotDst := binary.BigEndian.Uint16(data[2:4])
	if gotSrc != srcPort {
		t.Errorf("src port = %d, want %d", gotSrc, srcPort)
	}
	if gotDst != dstPort {
		t.Errorf("dst port = %d, want %d", gotDst, dstPort)
	}
}

func TestEncodeUDPPacketUDPLength(t *testing.T) {
	payload := make([]byte, 50)
	data, err := EncodeUDPPacket(
		[]byte{10, 0, 0, 1},
		[]byte{10, 0, 0, 2},
		1000, 2000, 0, 64, payload,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	udpLen := binary.BigEndian.Uint16(data[4:6])
	expectedLen := uint16(8 + len(payload))
	if udpLen != expectedLen {
		t.Errorf("UDP length field = %d, want %d", udpLen, expectedLen)
	}
}

func TestEncodeUDPPacketPayloadPreserved(t *testing.T) {
	payload := []byte("CHAOOAHC-test-data-here")
	data, err := EncodeUDPPacket(
		[]byte{192, 168, 1, 1},
		[]byte{10, 0, 0, 2},
		12345, 80, 0, 64, payload,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Payload starts after 8-byte UDP header
	if !bytes.Equal(data[8:], payload) {
		t.Errorf("payload mismatch:\n  got  %x\n  want %x", data[8:], payload)
	}
}

func TestEncodeUDPPacketEmptyPayload(t *testing.T) {
	data, err := EncodeUDPPacket(
		[]byte{127, 0, 0, 1},
		[]byte{127, 0, 0, 1},
		1000, 2000, 0, 64, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != 8 {
		t.Errorf("expected 8 bytes (UDP header only), got %d", len(data))
	}
}

func TestEncodeUDPPacketLargePayload(t *testing.T) {
	payload := make([]byte, 1400)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	data, err := EncodeUDPPacket(
		[]byte{192, 168, 1, 1},
		[]byte{10, 0, 0, 2},
		12345, 80, 0, 64, payload,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(data[8:], payload) {
		t.Error("large payload mismatch")
	}
}

func TestEncodeUDPPacketFullProbePacket(t *testing.T) {
	// Simulate a real probe packet like the client would send
	localIP := []byte{192, 168, 1, 100}
	remoteIP := []byte{10, 0, 0, 1}
	localPort := uint16(43500)
	remotePort := uint16(43500)
	tos := uint8(128)
	ttl := 64

	seq := uint64(1)
	ts := int64(1700000000_000000000)
	salt := bytes.Repeat([]byte{0xFF}, 32) // MsgHeaderLen + 32 = 64 byte msg
	payload := Encode(seq, salt, ts, MsgHeaderLen+len(salt), 100, 43500, 43500)

	data, err := EncodeUDPPacket(localIP, remoteIP, localPort, remotePort, tos, ttl, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify ports
	gotSrc := binary.BigEndian.Uint16(data[0:2])
	gotDst := binary.BigEndian.Uint16(data[2:4])
	if gotSrc != localPort {
		t.Errorf("src port = %d, want %d", gotSrc, localPort)
	}
	if gotDst != remotePort {
		t.Errorf("dst port = %d, want %d", gotDst, remotePort)
	}

	// Verify payload is preserved after UDP header
	if !bytes.Equal(data[8:], payload) {
		t.Error("probe payload not preserved in UDP packet")
	}

	// Verify the payload is still decodable
	appPayload := data[8:]
	if !IsValid(appPayload) {
		t.Error("embedded payload should be valid")
	}
	gotSeq, gotTS, gotLastSent, gotStartSrc, gotStartDst := Decode(appPayload)
	if gotSeq != seq || gotTS != ts || gotLastSent != 100 || gotStartSrc != 43500 || gotStartDst != 43500 {
		t.Errorf("decode embedded: seq=%d ts=%d lastSent=%d startSrc=%d startDst=%d", gotSeq, gotTS, gotLastSent, gotStartSrc, gotStartDst)
	}
}

// ---------------------------------------------------------------------------
// MsgHeaderLen constant
// ---------------------------------------------------------------------------

func TestMsgHeaderLenValue(t *testing.T) {
	// magicFlag(8) + seq(8) + ts(8) + lastSent(4) + lastStartSrcPort(2) + lastStartDstPort(2) = 32
	if MsgHeaderLen != 32 {
		t.Errorf("MsgHeaderLen = %d, want 32", MsgHeaderLen)
	}
}
