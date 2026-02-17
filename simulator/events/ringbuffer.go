package events

import "sync"

type RingBuffer struct {
	items []interface{}
	head  int
	count int
	cap   int
	mu    sync.RWMutex
}

func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		items: make([]interface{}, capacity),
		cap:   capacity,
	}
}

func (rb *RingBuffer) Push(item interface{}) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	idx := (rb.head + rb.count) % rb.cap
	rb.items[idx] = item
	if rb.count == rb.cap {
		rb.head = (rb.head + 1) % rb.cap
	} else {
		rb.count++
	}
}

func (rb *RingBuffer) GetAll() []interface{} {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	result := make([]interface{}, rb.count)
	for i := 0; i < rb.count; i++ {
		result[i] = rb.items[(rb.head+i)%rb.cap]
	}
	return result
}

func (rb *RingBuffer) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}
