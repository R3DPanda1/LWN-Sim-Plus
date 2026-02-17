package events

import (
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var eventCounter uint64

func nextID() string {
	n := atomic.AddUint64(&eventCounter, 1)
	return time.Now().Format("20060102150405") + "-" + strconv.FormatUint(n, 10)
}

type subscriber struct {
	ch     chan interface{}
	filter string
}

type EventBroker struct {
	store       *EventStore
	subscribers map[string][]*subscriber
	mu          sync.RWMutex
}

func NewEventBroker(maxHistoryPerDevice int) *EventBroker {
	return &EventBroker{
		store:       NewEventStore(maxHistoryPerDevice),
		subscribers: make(map[string][]*subscriber),
	}
}

func (b *EventBroker) Subscribe(topic string) (ch <-chan interface{}, history []interface{}, unsubscribe func()) {
	sub := &subscriber{
		ch:     make(chan interface{}, 256),
		filter: topic,
	}

	b.mu.Lock()
	b.subscribers[topic] = append(b.subscribers[topic], sub)
	b.mu.Unlock()

	history = b.store.GetHistory(topic)

	unsubscribe = func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		subs := b.subscribers[topic]
		for i, s := range subs {
			if s == sub {
				b.subscribers[topic] = append(subs[:i], subs[i+1:]...)
				close(sub.ch)
				break
			}
		}
	}

	return sub.ch, history, unsubscribe
}

func (b *EventBroker) publish(topic string, event interface{}) {
	b.store.Store(topic, event)

	b.mu.RLock()
	subs := b.subscribers[topic]
	b.mu.RUnlock()

	for _, sub := range subs {
		select {
		case sub.ch <- event:
		default:
			slog.Warn("event subscriber buffer full, dropping event", "topic", topic)
		}
	}
}

func (b *EventBroker) PublishDeviceEvent(devEUI string, event DeviceEvent) {
	if event.ID == "" {
		event.ID = nextID()
	}
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	b.publish(DeviceTopic(devEUI), event)
	if event.Type == EventError {
		b.publish(ErrorsTopic, event)
	}
}

func (b *EventBroker) PublishGatewayEvent(gwMAC string, event GatewayEvent) {
	if event.ID == "" {
		event.ID = nextID()
	}
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	b.publish(GatewayTopic(gwMAC), event)
	if event.Type == GwEventError {
		b.publish(ErrorsTopic, event)
	}
}

func (b *EventBroker) PublishSystemEvent(event SystemEvent) {
	if event.ID == "" {
		event.ID = nextID()
	}
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	b.publish(SystemTopic, event)
	if event.IsError {
		b.publish(ErrorsTopic, event)
	}
}

func (b *EventBroker) RemoveDevice(devEUI string) {
	topic := DeviceTopic(devEUI)
	b.store.Remove(topic)
	b.mu.Lock()
	if subs, ok := b.subscribers[topic]; ok {
		for _, sub := range subs {
			close(sub.ch)
		}
		delete(b.subscribers, topic)
	}
	b.mu.Unlock()
}

func (b *EventBroker) RemoveGateway(gwMAC string) {
	topic := GatewayTopic(gwMAC)
	b.store.Remove(topic)
	b.mu.Lock()
	if subs, ok := b.subscribers[topic]; ok {
		for _, sub := range subs {
			close(sub.ch)
		}
		delete(b.subscribers, topic)
	}
	b.mu.Unlock()
}
