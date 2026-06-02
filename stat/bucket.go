package stat

import (
	"fmt"
	"sync"
	"time"
)

const rttUnset = int64(-1)

// request tracks a single probe packet: its port pair, sequence, send
// timestamp, RTT (set on receive), and whether a bit-flip was detected.
type request struct {
	clientPort uint16
	serverPort uint16
	seq        uint64
	ts         int64
	rtt        int64
	hasBitflip bool
}

// buckets is a thread-safe collection of time-bucketed probe records.
// Each bucket corresponds to one span interval and is keyed by its
// bucket ID (timestamp / span). It tracks the oldest bucket ID
// explicitly to avoid linear scans of the map.
type buckets struct {
	sync.RWMutex
	buckets    map[int64]*bucket
	rateInSpan int64
	span       time.Duration
	minID      int64 // smallest bucket ID currently stored
	serverSide bool
}

// newBuckets creates a buckets collection with the given span and
// expected packets-per-span rate.
func newBuckets(span time.Duration, rateInSpan int64, serverSide bool) *buckets {
	return &buckets{
		buckets:    make(map[int64]*bucket),
		rateInSpan: rateInSpan,
		span:       span,
		minID:      -1,
		serverSide: serverSide,
	}
}

// newBucket creates a bucket for the given ID and span.
func newBucket(bucketID int64, span time.Duration, rateInSpan int64, serverSide bool) *bucket {
	return &bucket{
		id:          bucketID,
		startNano:   bucketID * int64(span),
		packetCount: uint32(rateInSpan),
		serverSide:  serverSide,
		requests:    make(map[uint64]request),
	}
}

// put records a sent probe in the bucket corresponding to the given timestamp.
func (bs *buckets) put(clientPort, serverPort uint16, seq uint64, ts int64) {
	bucketID := ts / int64(bs.span)

	bs.Lock()
	b := bs.buckets[bucketID]
	if b == nil {
		b = newBucket(bucketID, bs.span, bs.rateInSpan, bs.serverSide)
		bs.buckets[bucketID] = b
		if bs.minID < 0 || bucketID < bs.minID {
			bs.minID = bucketID
		}
	}
	bs.Unlock()

	b.put(clientPort, serverPort, seq, ts)
}

// delete removes a sent record from its bucket (used when the send fails).
func (bs *buckets) delete(seq uint64, ts int64) {
	bucketID := ts / int64(bs.span)

	bs.RLock()
	b := bs.buckets[bucketID]
	bs.RUnlock()

	if b != nil {
		b.delete(seq)
	}
}

// received marks a probe as received in the bucket corresponding to the
// given timestamp, recording its RTT and bit-flip status.
func (bs *buckets) received(seq uint64, ts, rtt int64, hasBitflip bool) {
	bucketID := ts / int64(bs.span)

	bs.Lock()
	b := bs.buckets[bucketID]
	if b == nil {
		b = newBucket(bucketID, bs.span, bs.rateInSpan, bs.serverSide)
		bs.buckets[bucketID] = b
		if bs.minID < 0 || bucketID < bs.minID {
			bs.minID = bucketID
		}
	}
	bs.Unlock()

	b.received(seq, rtt, hasBitflip)
}

// receivedAndFix marks a probe as received and, on the first call for a
// bucket, fixes the previous bucket's packetCount and starting ports.
// This corrects the denominator for loss-rate calculation when the server
// reports how many packets the client actually sent in the prior span.
func (bs *buckets) receivedAndFix(seq uint64, ts, rtt int64, lastSentCount uint32, lastStartSrcPort, lastStartDstPort uint16, hasBitflip bool) {
	bucketID := ts / int64(bs.span)

	bs.Lock()
	b := bs.buckets[bucketID]
	if b == nil {
		b = newBucket(bucketID, bs.span, bs.rateInSpan, bs.serverSide)
		bs.buckets[bucketID] = b
		if bs.minID < 0 || bucketID < bs.minID {
			bs.minID = bucketID
		}
	}
	b.received(seq, rtt, hasBitflip)
	fixed := b.fixed
	b.fixed = true
	bs.Unlock()

	if !fixed && lastSentCount > 0 {
		bs.RLock()
		prev := bs.buckets[bucketID-1]
		bs.RUnlock()
		if prev != nil {
			prev.Lock()
			prev.packetCount = lastSentCount
			prev.packetCountFixed = true
			prev.startSrcPort = lastStartSrcPort
			prev.startDstPort = lastStartDstPort
			prev.Unlock()
		}
	}
}

// oldest returns the bucket with the smallest bucket ID, or nil if empty.
func (bs *buckets) oldest() *bucket {
	bs.RLock()
	defer bs.RUnlock()
	if len(bs.buckets) == 0 {
		return nil
	}
	return bs.buckets[bs.minID]
}

// remove deletes the bucket with the given ID and updates minID.
func (bs *buckets) remove(bucketID int64) {
	bs.Lock()
	delete(bs.buckets, bucketID)
	// Update minID: if we removed the oldest, find the next one.
	if bucketID == bs.minID {
		bs.minID = -1
		for id := range bs.buckets {
			if bs.minID < 0 || id < bs.minID {
				bs.minID = id
			}
		}
	}
	bs.Unlock()
}

// bucket holds probe records for a single time span interval.
type bucket struct {
	id               int64
	startNano        int64 // approximate start time in nanoseconds for display
	packetCount      uint32
	packetCountFixed bool
	startSrcPort     uint16
	startDstPort     uint16
	fixed            bool
	serverSide       bool
	sync.RWMutex
	requests map[uint64]request
}

// put adds or updates a request entry keyed by sequence number.
func (b *bucket) put(clientPort, serverPort uint16, seq uint64, ts int64) {
	b.Lock()
	r, ok := b.requests[seq]
	if ok {
		r.clientPort = clientPort
		r.serverPort = serverPort
		r.ts = ts
		b.requests[seq] = r
	} else {
		b.requests[seq] = request{
			clientPort: clientPort,
			serverPort: serverPort,
			seq:        seq,
			ts:         ts,
			rtt:        rttUnset,
		}
	}
	b.Unlock()
}

// delete removes a request by sequence number.
func (b *bucket) delete(seq uint64) {
	b.Lock()
	delete(b.requests, seq)
	b.Unlock()
}

// received marks a request as received, setting its RTT and bit-flip status.
// If the request was not previously recorded (e.g. out-of-order receive),
// a new entry is created.
func (b *bucket) received(seq uint64, rtt int64, hasBitflip bool) {
	b.Lock()
	r, ok := b.requests[seq]
	if ok {
		r.rtt = rtt
		r.hasBitflip = hasBitflip
		b.requests[seq] = r
	} else {
		b.requests[seq] = request{seq: seq, rtt: rtt, hasBitflip: hasBitflip}
	}
	b.Unlock()
}

// stat computes aggregated statistics for all requests in the bucket:
// sent/received counts, loss rate, average and max RTT, and per-port
// breakdowns of lost and bit-flipped packets.
func (b *bucket) stat() statResult {
	b.RLock()
	defer b.RUnlock()

	if len(b.requests) == 0 {
		return statResult{}
	}

	if b.serverSide {
		return b.serverStat()
	}

	result := statResult{
		lossPorts:         make(map[int]int),
		bitflipPorts:      make(map[int]int),
		lossPortsCount:    make(map[string]int),
		bitflipPortsCount: make(map[string]int),
	}

	var portKeyBuf [32]byte

	for _, r := range b.requests {
		result.sent++
		srcPort := int(r.clientPort)
		dstPort := int(r.serverPort)

		if r.rtt != rttUnset {
			result.received++
			result.rtt += r.rtt
			if r.hasBitflip {
				result.bitflipPorts[srcPort] = dstPort
				key := fmt.Appendf(portKeyBuf[:0], "%d-%d", r.clientPort, r.serverPort)
				result.bitflipPortsCount[string(key)]++
			}
			if r.rtt > result.maxRTT {
				result.maxRTT = r.rtt
			}
		} else {
			result.lossPorts[srcPort] = dstPort
			key := fmt.Appendf(portKeyBuf[:0], "%d-%d", r.clientPort, r.serverPort)
			result.lossPortsCount[string(key)]++
		}
	}

	result.loss = result.sent - result.received
	if result.sent > 0 {
		result.lossRate = float64(result.loss) / float64(result.sent)
	}
	if result.received > 0 {
		result.rtt /= int64(result.received)
	}
	return result
}

func (b *bucket) serverStat() statResult {
	sent := int(b.packetCount)
	if !b.packetCountFixed {
		sent = len(b.requests)
	}

	result := statResult{
		sent: sent,
	}

	for _, r := range b.requests {
		if r.rtt != rttUnset {
			result.received++
			result.rtt += r.rtt
			if r.rtt > result.maxRTT {
				result.maxRTT = r.rtt
			}
		}
	}

	result.loss = result.sent - result.received
	if result.loss < 0 {
		result.loss = 0
	}
	if result.sent > 0 {
		result.lossRate = float64(result.loss) / float64(result.sent)
	}
	if result.received > 0 {
		result.rtt /= int64(result.received)
	}
	return result
}
