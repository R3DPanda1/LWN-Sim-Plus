package events

import "testing"

func TestRingBufferBasic(t *testing.T) {
	rb := NewRingBuffer(3)
	rb.Push("a")
	rb.Push("b")

	items := rb.GetAll()
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0] != "a" || items[1] != "b" {
		t.Errorf("expected [a, b], got %v", items)
	}
}

func TestRingBufferOverflow(t *testing.T) {
	rb := NewRingBuffer(3)
	rb.Push("a")
	rb.Push("b")
	rb.Push("c")
	rb.Push("d") // overwrites "a"

	items := rb.GetAll()
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0] != "b" || items[1] != "c" || items[2] != "d" {
		t.Errorf("expected [b, c, d], got %v", items)
	}
}

func TestRingBufferEmpty(t *testing.T) {
	rb := NewRingBuffer(5)
	items := rb.GetAll()
	if len(items) != 0 {
		t.Errorf("expected empty, got %d items", len(items))
	}
}
