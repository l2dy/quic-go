package ackhandler

import (
	"time"

	"github.com/lucas-clemente/quic-go/internal/protocol"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pacer", func() {
	var p *pacer

	const packetsPerSecond = 42

	BeforeEach(func() {
		p = newPacer(packetsPerSecond * maxPacketSize) // bandwidth: 42 full-size packets per second
	})

	It("allows a burst at the beginning", func() {
		t := time.Now()
		Expect(p.TimeUntilSend(t)).To(BeZero())
		Expect(p.Budget(t)).To(BeEquivalentTo(maxBurstSize))
	})

	It("reduces the budget when sending packets", func() {
		t := time.Now()
		budget := p.Budget(t)
		for budget > 0 {
			Expect(p.TimeUntilSend(t)).To(BeZero())
			Expect(p.Budget(t)).To(Equal(budget))
			p.SentPacket(&Packet{Length: maxPacketSize, SendTime: t})
			budget -= maxPacketSize
		}
		Expect(p.Budget(t)).To(BeZero())
		Expect(p.TimeUntilSend(t)).ToNot(BeZero())
	})

	sendBurst := func(t time.Time) {
		for p.Budget(t) > 0 {
			p.SentPacket(&Packet{Length: maxPacketSize, SendTime: t})
		}
	}

	It("paces packets after a burst", func() {
		t := time.Now()
		sendBurst(t)
		// send 100 exactly paced packets
		for i := 0; i < 100; i++ {
			dur := p.TimeUntilSend(t)
			Expect(dur).To(BeNumerically("~", time.Second/packetsPerSecond, time.Nanosecond))
			t = t.Add(dur)
			Expect(p.Budget(t)).To(BeEquivalentTo(maxPacketSize))
			p.SentPacket(&Packet{Length: maxPacketSize, SendTime: t})
		}
	})

	It("accounts for non-full-size packets", func() {
		t := time.Now()
		sendBurst(t)
		dur := p.TimeUntilSend(t)
		Expect(dur).To(BeNumerically("~", time.Second/packetsPerSecond, time.Nanosecond))
		// send a half-full packet
		t = t.Add(dur)
		Expect(p.Budget(t)).To(BeEquivalentTo(maxPacketSize))
		size := protocol.ByteCount(maxPacketSize / 2)
		p.SentPacket(&Packet{Length: size, SendTime: t})
		Expect(p.Budget(t)).To(Equal(maxPacketSize - size))
		Expect(p.TimeUntilSend(t)).To(BeNumerically("~", time.Second/packetsPerSecond/2, time.Nanosecond))
	})

	It("accumulates budget, if no packets are sent", func() {
		t := time.Now()
		sendBurst(t)
		dur := p.TimeUntilSend(t)
		Expect(dur).ToNot(BeZero())
		// wait for 5 times the duration
		Expect(p.Budget(t.Add(5 * dur))).To(BeEquivalentTo(5 * maxPacketSize))
	})

	It("never allows bursts larger than the maximum burst size", func() {
		t := time.Now()
		sendBurst(t)
		Expect(p.Budget(t.Add(time.Hour))).To(BeEquivalentTo(maxBurstSize))
	})
})
