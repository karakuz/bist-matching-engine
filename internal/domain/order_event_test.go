package domain

import (
	"testing"
	"time"
)

func TestNewOrderEvent_AcceptsValidEvent(t *testing.T) {
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	payload := []byte(`{"orderId":"ord_1"}`)

	event, err := NewOrderEvent("ord_1", EventOrderAccepted, payload, now)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if event.ID != 0 {
		t.Fatalf("expected ID to be 0 before DB insert, got %d", event.ID)
	}
	if event.OrderID != "ord_1" {
		t.Fatalf("expected OrderID ord_1, got %s", event.OrderID)
	}
	if event.EventType != EventOrderAccepted {
		t.Fatalf("expected EventType %s, got %s", EventOrderAccepted, event.EventType)
	}
	if string(event.Payload) != string(payload) {
		t.Fatalf("expected payload %s, got %s", payload, event.Payload)
	}
	if !event.CreatedAt.Equal(now) {
		t.Fatalf("expected CreatedAt %v, got %v", now, event.CreatedAt)
	}
}

func TestNewOrderEvent_DefaultsCreatedAt(t *testing.T) {
	event, err := NewOrderEvent("ord_1", EventOrderAccepted, []byte(`{}`), time.Time{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if event.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}
}