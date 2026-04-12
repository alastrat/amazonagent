package service

import (
	"testing"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

func makeTestEvent(msg string) ChatEvent {
	return ChatEvent{
		Type:      ChatEventMessage,
		Timestamp: time.Now(),
		Data:      map[string]any{"content": msg},
	}
}

func TestChatHub_PublishSubscriberReceives(t *testing.T) {
	hub := NewChatHub()
	tenantID := domain.TenantID("tenant-hub-1")

	ch, unsub := hub.Subscribe(tenantID)
	defer unsub()

	evt := makeTestEvent("hello")
	hub.Publish(tenantID, evt)

	select {
	case got := <-ch:
		if got.Data["content"] != "hello" {
			t.Errorf("content = %v, want %q", got.Data["content"], "hello")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestChatHub_MultipleSubscribers(t *testing.T) {
	hub := NewChatHub()
	tenantID := domain.TenantID("tenant-hub-2")

	ch1, unsub1 := hub.Subscribe(tenantID)
	defer unsub1()
	ch2, unsub2 := hub.Subscribe(tenantID)
	defer unsub2()

	evt := makeTestEvent("broadcast")
	hub.Publish(tenantID, evt)

	for i, ch := range []chan ChatEvent{ch1, ch2} {
		select {
		case got := <-ch:
			if got.Data["content"] != "broadcast" {
				t.Errorf("subscriber %d: content = %v, want %q", i, got.Data["content"], "broadcast")
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out waiting for event", i)
		}
	}
}

func TestChatHub_UnsubscribeStopsDelivery(t *testing.T) {
	hub := NewChatHub()
	tenantID := domain.TenantID("tenant-hub-3")

	ch, unsub := hub.Subscribe(tenantID)
	unsub()

	hub.Publish(tenantID, makeTestEvent("after-unsub"))

	select {
	case evt := <-ch:
		t.Errorf("should not receive after unsubscribe, got %v", evt)
	case <-time.After(50 * time.Millisecond):
		// expected: no event received
	}
}

func TestChatHub_PublishNoSubscribersNoPanic(t *testing.T) {
	hub := NewChatHub()
	tenantID := domain.TenantID("tenant-hub-4")

	// Should not panic
	hub.Publish(tenantID, makeTestEvent("nobody listening"))
}

func TestChatHub_TenantIsolation(t *testing.T) {
	hub := NewChatHub()
	tenantA := domain.TenantID("tenant-A")
	tenantB := domain.TenantID("tenant-B")

	chA, unsubA := hub.Subscribe(tenantA)
	defer unsubA()
	chB, unsubB := hub.Subscribe(tenantB)
	defer unsubB()

	hub.Publish(tenantA, makeTestEvent("for-A"))

	select {
	case got := <-chA:
		if got.Data["content"] != "for-A" {
			t.Errorf("tenant A: content = %v, want %q", got.Data["content"], "for-A")
		}
	case <-time.After(time.Second):
		t.Fatal("tenant A should have received the event")
	}

	select {
	case evt := <-chB:
		t.Errorf("tenant B should not receive tenant A's event, got %v", evt)
	case <-time.After(50 * time.Millisecond):
		// expected: tenant B receives nothing
	}
}

func TestChatHub_SlowSubscriberDoesNotBlock(t *testing.T) {
	hub := NewChatHub()
	tenantID := domain.TenantID("tenant-hub-slow")

	ch, unsub := hub.Subscribe(tenantID)
	defer unsub()

	// Fill the channel buffer (capacity is 32)
	for i := 0; i < 40; i++ {
		hub.Publish(tenantID, makeTestEvent("flood"))
	}

	// Drain what we can
	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto done
		}
	}
done:
	// We should have received at most the buffer size (32), and the rest were dropped
	if count > 32 {
		t.Errorf("received %d events, expected at most 32 (buffer size)", count)
	}
	if count == 0 {
		t.Error("expected to receive some events")
	}
}
