package events

import (
	"testing"
	"time"
)

func TestBrokerPublishSubscribe(t *testing.T) {
	broker := NewEventBroker(100)
	ch, history, unsub := broker.Subscribe(DeviceTopic("0102030405060708"))
	defer unsub()

	if len(history) != 0 {
		t.Errorf("expected empty history, got %d", len(history))
	}

	broker.PublishDeviceEvent("0102030405060708", DeviceEvent{
		DevEUI:  "0102030405060708",
		DevName: "test-device",
		Type:    EventUp,
	})

	select {
	case event := <-ch:
		de := event.(DeviceEvent)
		if de.Type != EventUp {
			t.Errorf("expected 'up' event, got %s", de.Type)
		}
		if de.ID == "" {
			t.Error("expected auto-generated ID")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestBrokerHistory(t *testing.T) {
	broker := NewEventBroker(100)

	broker.PublishDeviceEvent("aabbccdd", DeviceEvent{Type: EventJoin})
	broker.PublishDeviceEvent("aabbccdd", DeviceEvent{Type: EventUp})

	_, history, unsub := broker.Subscribe(DeviceTopic("aabbccdd"))
	defer unsub()

	if len(history) != 2 {
		t.Fatalf("expected 2 history events, got %d", len(history))
	}
}

func TestBrokerErrorsAutoPublish(t *testing.T) {
	broker := NewEventBroker(100)
	errCh, _, unsub := broker.Subscribe(ErrorsTopic)
	defer unsub()

	broker.PublishDeviceEvent("aabbccdd", DeviceEvent{Type: EventError})

	select {
	case <-errCh:
		// good
	case <-time.After(time.Second):
		t.Fatal("error event not published to errors topic")
	}
}

func TestBrokerUnsubscribe(t *testing.T) {
	broker := NewEventBroker(100)
	ch, _, unsub := broker.Subscribe(DeviceTopic("aabbccdd"))
	unsub()

	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after unsubscribe")
	}
}
