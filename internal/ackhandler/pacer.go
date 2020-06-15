package ackhandler

import (
	"math"
	"time"

	"github.com/lucas-clemente/quic-go/internal/protocol"
	"github.com/lucas-clemente/quic-go/internal/utils"
)

const (
	maxPacketSize = protocol.MaxPacketSizeIPv4
	maxBurstSize  = 10 * maxPacketSize
)

// The pacer implements a leaky-bucket pacing algorithm.
type pacer struct {
	budgetAtLastSent protocol.ByteCount
	lastSentTime     time.Time
	bandwidth        uint64 // in bytes / s
}

func newPacer(bw uint64) *pacer {
	return &pacer{
		bandwidth:        bw,
		budgetAtLastSent: maxBurstSize,
	}
}

func (p *pacer) SentPacket(packet *Packet) {
	budget := p.Budget(packet.SendTime)
	if packet.Length > budget {
		p.budgetAtLastSent = 0
	} else {
		p.budgetAtLastSent = budget - packet.Length
	}
	p.lastSentTime = packet.SendTime
}

func (p *pacer) SetBandwidth(bw uint64) {
	if bw == 0 {
		panic("zero bandwidth")
	}
	p.bandwidth = bw
}

func (p *pacer) Budget(now time.Time) protocol.ByteCount {
	if p.lastSentTime.IsZero() {
		return p.budgetAtLastSent
	}
	budget := p.budgetAtLastSent + (protocol.ByteCount(p.bandwidth)*protocol.ByteCount(now.Sub(p.lastSentTime).Nanoseconds()))/1e9
	return utils.MinByteCount(maxBurstSize, budget)
}

// TimeUntilSend returns when the next packet should be sent.
func (p *pacer) TimeUntilSend(now time.Time) time.Duration {
	if p.budgetAtLastSent >= maxPacketSize {
		return 0
	}
	// TODO: don't allow pacing faster than MinPacingDelay
	return time.Duration(math.Ceil(float64(maxPacketSize-p.budgetAtLastSent)*1e9/float64(p.bandwidth))) * time.Nanosecond
}
