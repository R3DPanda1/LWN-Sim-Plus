package events

import "sync"

type EventStore struct {
	buffers    map[string]*RingBuffer
	maxHistory int
	mu         sync.RWMutex
}

func NewEventStore(maxHistory int) *EventStore {
	return &EventStore{
		buffers:    make(map[string]*RingBuffer),
		maxHistory: maxHistory,
	}
}

func (s *EventStore) Store(topic string, event interface{}) {
	s.mu.Lock()
	buf, ok := s.buffers[topic]
	if !ok {
		buf = NewRingBuffer(s.maxHistory)
		s.buffers[topic] = buf
	}
	s.mu.Unlock()
	buf.Push(event)
}

func (s *EventStore) GetHistory(topic string) []interface{} {
	s.mu.RLock()
	buf, ok := s.buffers[topic]
	s.mu.RUnlock()
	if !ok {
		return nil
	}
	return buf.GetAll()
}

func (s *EventStore) Remove(topic string) {
	s.mu.Lock()
	delete(s.buffers, topic)
	s.mu.Unlock()
}
