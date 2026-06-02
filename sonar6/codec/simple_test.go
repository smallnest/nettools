package codec

import (
	"encoding/binary"
	"net"
	"testing"
)

// buildIPv6Header constructs a 40-byte IPv6 fixed header.
// nextHeader is placed at byte 6; other fields are zeroed.
func buildIPv6Header(nextHeader uint8) []byte {
	h := make([]byte, 40)
	// Version=6 in the high nibble of byte 0
	h[0] = 6 << 4
	h[6] = nextHeader
	return h
}

// buildExtHeader constructs a standard IPv6 extension header (NextHeader/HdrExtLen format).
// nextHeader goes in byte 0, HdrExtLen is computed from padLen (additional 8-byte units beyond the first).
// The header total length is (padLen+1)*8 bytes.
func buildExtHeader(nextHeader uint8, padLen uint8) []byte {
	size := int(padLen+1) * 8
	h := make([]byte, size)
	h[0] = nextHeader
	h[1] = padLen
	return h
}

func TestEncodeUDPPacket_IPv6Checksum(t *testing.T) {
	localIP := net.ParseIP("2001:db8::1")
	remoteIP := net.ParseIP("2001:db8::2")
	payload := []byte("hello-ipv6")

	data, err := EncodeUDPPacket(localIP, remoteIP, 12345, 80, 0, 64, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != 8+len(payload) {
		t.Errorf("len = %d, want %d", len(data), 8+len(payload))
	}
	// Verify checksum is non-zero (mandatory for IPv6 UDP per RFC 2460)
	checksum := binary.BigEndian.Uint16(data[6:8])
	if checksum == 0 {
		t.Error("IPv6 UDP checksum must not be zero (RFC 2460)")
	}
}

func TestEncodeUDPPacket_IPv4AddressRejected(t *testing.T) {
	localIP := net.IP{192, 168, 1, 1}
	remoteIP := net.IP{10, 0, 0, 2}
	_, err := EncodeUDPPacket(localIP, remoteIP, 12345, 80, 0, 64, []byte("test"))
	if err == nil {
		t.Error("expected error for IPv4 address passed to IPv6 encoder")
	}
}

func TestReExports(t *testing.T) {
	payload := Encode(1, nil, 0, MsgHeaderLen, 0, 0, 0)
	if !IsValid(payload) {
		t.Error("re-exported IsValid should work")
	}
	seq, ts, lastSent, srcPort, dstPort := Decode(payload)
	if seq != 1 || ts != 0 || lastSent != 0 || srcPort != 0 || dstPort != 0 {
		t.Errorf("re-exported Decode: seq=%d ts=%d lastSent=%d srcPort=%d dstPort=%d", seq, ts, lastSent, srcPort, dstPort)
	}
}

func TestEncodeUDPPacket_Ports(t *testing.T) {
	localIP := net.ParseIP("::1")
	remoteIP := net.ParseIP("::1")
	srcPort := uint16(43500)
	dstPort := uint16(43509)

	data, err := EncodeUDPPacket(localIP, remoteIP, srcPort, dstPort, 0, 64, []byte("test"))
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

func TestEncodeUDPPacket_PayloadPreserved(t *testing.T) {
	localIP := net.ParseIP("2001:db8::1")
	remoteIP := net.ParseIP("2001:db8::2")
	payload := []byte("CHAOOAHC-ipv6-test")

	data, err := EncodeUDPPacket(localIP, remoteIP, 12345, 80, 0, 64, payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != 8+len(payload) {
		t.Errorf("len = %d, want %d", len(data), 8+len(payload))
	}
}

func TestEncodeUDPPacket_EmptyPayload(t *testing.T) {
	localIP := net.ParseIP("::1")
	remoteIP := net.ParseIP("::1")

	data, err := EncodeUDPPacket(localIP, remoteIP, 1000, 2000, 0, 64, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != 8 {
		t.Errorf("expected 8 bytes (UDP header only), got %d", len(data))
	}
}

func TestEncodeUDPPacket_ShortAddressRejected(t *testing.T) {
	// nil address
	_, err := EncodeUDPPacket(nil, net.ParseIP("::1"), 12345, 80, 0, 64, []byte("test"))
	if err == nil {
		t.Error("expected error for nil localIP")
	}

	// 4-byte remote
	_, err = EncodeUDPPacket(net.ParseIP("::1"), net.IP{10, 0, 0, 1}, 12345, 80, 0, 64, []byte("test"))
	if err == nil {
		t.Error("expected error for 4-byte remoteIP")
	}
}

func TestParseExtensionHeaders_NoExtensionHeaders(t *testing.T) {
	// NextHeader=17 (UDP) directly — no extension headers
	pkt := buildIPv6Header(17)
	nh, off, err := ParseExtensionHeaders(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nh != 17 {
		t.Errorf("nextHeader = %d, want 17", nh)
	}
	if off != 40 {
		t.Errorf("offset = %d, want 40", off)
	}
}

func TestParseExtensionHeaders_HopByHop(t *testing.T) {
	// IPv6 header: NextHeader=0 (Hop-by-Hop)
	// Hop-by-Hop: NextHeader=17 (UDP), HdrExtLen=0 → 8 bytes
	ipHdr := buildIPv6Header(0)
	extHdr := buildExtHeader(17, 0) // 8 bytes
	pkt := append(ipHdr, extHdr...)

	nh, off, err := ParseExtensionHeaders(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nh != 17 {
		t.Errorf("nextHeader = %d, want 17", nh)
	}
	if off != 48 { // 40 + 8
		t.Errorf("offset = %d, want 48", off)
	}
}

func TestParseExtensionHeaders_Routing(t *testing.T) {
	// IPv6 header: NextHeader=43 (Routing)
	// Routing: NextHeader=6 (TCP), HdrExtLen=1 → 16 bytes
	ipHdr := buildIPv6Header(43)
	extHdr := buildExtHeader(6, 1) // 16 bytes
	pkt := append(ipHdr, extHdr...)

	nh, off, err := ParseExtensionHeaders(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nh != 6 {
		t.Errorf("nextHeader = %d, want 6", nh)
	}
	if off != 56 { // 40 + 16
		t.Errorf("offset = %d, want 56", off)
	}
}

func TestParseExtensionHeaders_Chain(t *testing.T) {
	// Hop-by-Hop → Routing → UDP
	ipHdr := buildIPv6Header(0)
	hbh := buildExtHeader(43, 0) // 8 bytes, next=Routing
	rth := buildExtHeader(17, 1) // 16 bytes, next=UDP
	pkt := append(ipHdr, hbh...)
	pkt = append(pkt, rth...)

	nh, off, err := ParseExtensionHeaders(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nh != 17 {
		t.Errorf("nextHeader = %d, want 17", nh)
	}
	if off != 64 { // 40 + 8 + 16
		t.Errorf("offset = %d, want 64", off)
	}
}

func TestParseExtensionHeaders_DestinationOptions(t *testing.T) {
	// Destination Options (60) → ICMPv6 (58)
	ipHdr := buildIPv6Header(60)
	extHdr := buildExtHeader(58, 0) // 8 bytes
	pkt := append(ipHdr, extHdr...)

	nh, off, err := ParseExtensionHeaders(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nh != 58 {
		t.Errorf("nextHeader = %d, want 58", nh)
	}
	if off != 48 {
		t.Errorf("offset = %d, want 48", off)
	}
}

func TestParseExtensionHeaders_Fragment(t *testing.T) {
	// Fragment (44) → UDP (17), fixed 8 bytes
	ipHdr := buildIPv6Header(44)
	frag := make([]byte, 8)
	frag[0] = 17 // next header = UDP
	pkt := append(ipHdr, frag...)

	nh, off, err := ParseExtensionHeaders(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nh != 17 {
		t.Errorf("nextHeader = %d, want 17", nh)
	}
	if off != 48 {
		t.Errorf("offset = %d, want 48", off)
	}
}

func TestParseExtensionHeaders_NoNextHeader(t *testing.T) {
	// NextHeader=59 (No Next Header)
	pkt := buildIPv6Header(59)
	nh, off, err := ParseExtensionHeaders(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nh != 59 {
		t.Errorf("nextHeader = %d, want 59", nh)
	}
	if off != 40 {
		t.Errorf("offset = %d, want 40", off)
	}
}

func TestParseExtensionHeaders_UnknownLogged(t *testing.T) {
	// NextHeader=253 (experimental) — should log and try standard format
	ipHdr := buildIPv6Header(253)
	extHdr := buildExtHeader(17, 0) // 8 bytes, next=UDP
	pkt := append(ipHdr, extHdr...)

	nh, off, err := ParseExtensionHeaders(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nh != 17 {
		t.Errorf("nextHeader = %d, want 17", nh)
	}
	if off != 48 {
		t.Errorf("offset = %d, want 48", off)
	}
}

func TestParseExtensionHeaders_HeaderTooShort(t *testing.T) {
	_, _, err := ParseExtensionHeaders([]byte{0x60, 0, 0, 0}) // only 4 bytes
	if err == nil {
		t.Error("expected error for short IPv6 header")
	}
}

func TestParseExtensionHeaders_TruncatedExtHeader(t *testing.T) {
	// IPv6 header says NextHeader=0 (Hop-by-Hop) but no extension header data follows
	ipHdr := buildIPv6Header(0)
	// Only 1 byte of extension header — not enough for NextHeader + HdrExtLen
	pkt := append(ipHdr, 0x11)

	_, _, err := ParseExtensionHeaders(pkt)
	if err == nil {
		t.Error("expected error for truncated extension header")
	}
}

func TestParseExtensionHeaders_TruncatedExtHeaderData(t *testing.T) {
	// HdrExtLen says 16 bytes but only 8 bytes available
	ipHdr := buildIPv6Header(0)
	extHdr := buildExtHeader(17, 1)     // claims 16 bytes
	pkt := append(ipHdr, extHdr[:8]...) // only provide 8

	_, _, err := ParseExtensionHeaders(pkt)
	if err == nil {
		t.Error("expected error for truncated extension header data")
	}
}

func TestParseExtensionHeaders_TruncatedFragment(t *testing.T) {
	ipHdr := buildIPv6Header(44)
	pkt := append(ipHdr, []byte{17, 0, 0, 0}...) // only 4 bytes, need 8

	_, _, err := ParseExtensionHeaders(pkt)
	if err == nil {
		t.Error("expected error for truncated fragment header")
	}
}

func TestParseExtensionHeaders_LargeHdrExtLen(t *testing.T) {
	// HdrExtLen=3 → (3+1)*8 = 32 bytes
	ipHdr := buildIPv6Header(0)
	extHdr := buildExtHeader(17, 3) // 32 bytes
	pkt := append(ipHdr, extHdr...)

	nh, off, err := ParseExtensionHeaders(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nh != 17 {
		t.Errorf("nextHeader = %d, want 17", nh)
	}
	if off != 72 { // 40 + 32
		t.Errorf("offset = %d, want 72", off)
	}
}

func TestParseExtensionHeaders_TCPDirect(t *testing.T) {
	pkt := buildIPv6Header(6) // TCP directly
	nh, off, err := ParseExtensionHeaders(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nh != 6 {
		t.Errorf("nextHeader = %d, want 6", nh)
	}
	if off != 40 {
		t.Errorf("offset = %d, want 40", off)
	}
}

func TestParseExtensionHeaders_ICMPv6Direct(t *testing.T) {
	pkt := buildIPv6Header(58) // ICMPv6 directly
	nh, _, err := ParseExtensionHeaders(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nh != 58 {
		t.Errorf("nextHeader = %d, want 58", nh)
	}
}

func TestParseExtensionHeaders_ThreeChain(t *testing.T) {
	// Hop-by-Hop → Routing → Destination Options → UDP
	ipHdr := buildIPv6Header(0)
	hbh := buildExtHeader(43, 0) // 8 bytes, next=Routing
	rth := buildExtHeader(60, 0) // 8 bytes, next=DestOpts
	dst := buildExtHeader(17, 0) // 8 bytes, next=UDP
	pkt := append(ipHdr, hbh...)
	pkt = append(pkt, rth...)
	pkt = append(pkt, dst...)

	nh, off, err := ParseExtensionHeaders(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nh != 17 {
		t.Errorf("nextHeader = %d, want 17", nh)
	}
	if off != 64 { // 40 + 8 + 8 + 8
		t.Errorf("offset = %d, want 64", off)
	}
}
