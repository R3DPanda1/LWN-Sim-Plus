package buffer

import (
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources/communication/packets"
)

const DefaultBufferSize = 1000

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
	default:
		// buffer full -- drop oldest, push new
		select {
		case <-bu.ch:
		default:
		}
		bu.ch <- rxpk
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
