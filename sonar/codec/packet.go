// Package codec implements encoding and decoding for UDP probe packets
// used by the bitflip detection tool.
//
// Each packet consists of a magic flag header followed by a sequence number,
// timestamp, last-sent count, and a salt-padded payload. The salt pattern
// is selected by seq%4, enabling bit-flip detection across four distinct
// byte patterns: 0xFF, 0x00, 0x5A, and complementary alternating (0xAA/0x55).
package codec

import (
	"bytes"
	"encoding/binary"

	"github.com/baidu/nettools/util"
)

const (
	magicFlagLen = 8
	MsgHeaderLen = magicFlagLen + 24 // magic + seq + ts + lastSent + lastStartSrcPort + lastStartDstPort
)

var magicFlag = []byte("CHAOOAHC")

// Encode builds a probe packet with the given sequence, salt padding,
// timestamp, last-sent count, and last span's starting ports.
// The resulting byte slice has length msgLen.
func Encode(seq uint64, salt []byte, ts int64, msgLen int, lastSentCount uint32, lastStartSrcPort, lastStartDstPort uint16) []byte {
	if msgLen < 32 {
		msgLen = 32
	}
	saltLen := msgLen - MsgHeaderLen
	data := make([]byte, msgLen)
	copy(data, magicFlag)
	binary.BigEndian.PutUint64(data[magicFlagLen:magicFlagLen+8], seq)
	binary.BigEndian.PutUint64(data[magicFlagLen+8:magicFlagLen+16], uint64(ts))
	binary.BigEndian.PutUint32(data[magicFlagLen+16:magicFlagLen+20], lastSentCount)
	binary.BigEndian.PutUint16(data[magicFlagLen+20:magicFlagLen+22], lastStartSrcPort)
	binary.BigEndian.PutUint16(data[magicFlagLen+22:magicFlagLen+24], lastStartDstPort)
	if saltLen > 0 {
		copy(data[magicFlagLen+24:], []byte(salt))
	}
	return data
}

// Decode extracts the sequence number, timestamp, last-sent count, and
// last span's starting ports from a probe packet payload.
// Returns zero values if data is too short.
func Decode(data []byte) (seq uint64, ts int64, lastSentCount uint32, lastStartSrcPort, lastStartDstPort uint16) {
	if len(data) < MsgHeaderLen {
		return
	}
	seq = binary.BigEndian.Uint64(data[magicFlagLen : magicFlagLen+8])
	ts = int64(binary.BigEndian.Uint64(data[magicFlagLen+8 : magicFlagLen+16]))
	lastSentCount = binary.BigEndian.Uint32(data[magicFlagLen+16 : magicFlagLen+20])
	lastStartSrcPort = binary.BigEndian.Uint16(data[magicFlagLen+20 : magicFlagLen+22])
	lastStartDstPort = binary.BigEndian.Uint16(data[magicFlagLen+22 : magicFlagLen+24])
	return
}

// IsValid checks whether the given byte slice starts with the expected
// magic flag and is long enough to contain a full header.
func IsValid(data []byte) bool {
	if len(data) < magicFlagLen+24 {
		return false
	}
	return bytes.Equal(magicFlag, data[:magicFlagLen])
}

// ComplementaryBytes returns n bytes of alternating complementary 16-bit words.
// Delegated to util.ComplementaryBytes.
var ComplementaryBytes = util.ComplementaryBytes
