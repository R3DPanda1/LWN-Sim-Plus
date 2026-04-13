package buffer

import (
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources/communication/packets"
)

const DefaultBufferSize = 100000

type BufferUplink struct {
	ch   chan packets.RXPK
	done chan struct{}
}

func NewBufferUplink(size int) *BufferUplink {
	if size <= 0 {
		size = DefaultBufferSize
	}
	return &BufferUplink{
		ch:   make(chan packets.RXPK, size),
		done: make(chan struct{}),
	}
}

func (bu *BufferUplink) Push(rxpk packets.RXPK) {
	select {
	case bu.ch <- rxpk:
		return
	default:
	}
	// buffer full — drop oldest, retry non-blocking
	select {
	case <-bu.ch:
	default:
	}
	select {
	case bu.ch <- rxpk:
	default:
		// still full (race with other pushers), drop this packet
	}
}

func (bu *BufferUplink) Pop() (packets.RXPK, bool) {
	select {
	case rxpk := <-bu.ch:
		return rxpk, true
	case <-bu.done:
		return packets.RXPK{}, false
	}
}

func (bu *BufferUplink) Signal() {
	select {
	case bu.done <- struct{}{}:
	default:
	}
}

func (bu *BufferUplink) Close() {
	close(bu.done)
}
