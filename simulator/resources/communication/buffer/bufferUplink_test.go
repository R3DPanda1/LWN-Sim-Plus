package buffer

import (
	"testing"
	"time"

	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources/communication/packets"
)

func TestBufferPushPop(t *testing.T) {
	buf := NewBufferUplink(10)
	buf.Push(packets.RXPK{Data: "test"})

	rxpk, ok := buf.Pop()
	if !ok {
		t.Fatal("expected ok=true")
	}
	if rxpk.Data != "test" {
		t.Errorf("expected 'test', got '%s'", rxpk.Data)
	}
}

func TestBufferClose(t *testing.T) {
	buf := NewBufferUplink(10)
	buf.Close()

	_, ok := buf.Pop()
	if ok {
		t.Error("expected ok=false after close")
	}
}

func TestBufferBackpressure(t *testing.T) {
	buf := NewBufferUplink(2)
	buf.Push(packets.RXPK{Data: "a"})
	buf.Push(packets.RXPK{Data: "b"})
	buf.Push(packets.RXPK{Data: "c"}) // should drop "a"

	rxpk, _ := buf.Pop()
	if rxpk.Data != "b" {
		t.Errorf("expected 'b' after overflow, got '%s'", rxpk.Data)
	}
}

func TestBufferSignal(t *testing.T) {
	buf := NewBufferUplink(10)

	done := make(chan bool)
	go func() {
		_, ok := buf.Pop()
		done <- ok
	}()

	time.Sleep(10 * time.Millisecond)
	buf.Signal()

	select {
	case ok := <-done:
		if ok {
			t.Error("expected ok=false after signal")
		}
	case <-time.After(time.Second):
		t.Fatal("Pop did not return after Signal")
	}
}
