package storage

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"bist-matching-engine/internal/domain"

	"github.com/google/uuid"
)

func assertJSONEqual(t *testing.T, want []byte, got []byte) {
	t.Helper()

	var wantJSON any
	if err := json.Unmarshal(want, &wantJSON); err != nil {
		t.Fatalf("expected valid want json, got %v", err)
	}

	var gotJSON any
	if err := json.Unmarshal(got, &gotJSON); err != nil {
		t.Fatalf("expected valid got json, got %v", err)
	}

	if !reflect.DeepEqual(wantJSON, gotJSON) {
		t.Fatalf("expected payload %+v, got %+v", wantJSON, gotJSON)
	}
}

func TestPostgresStore_InsertOrderEvent(t *testing.T) {
	store, ctx := newTestStore(t)

	event := domain.OrderEvent{
		OrderID:   uuid.NewString(),
		EventType: domain.EventOrderAccepted,
		Payload:   []byte(`{"status":"OPEN"}`),
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}

	id, err := store.InsertOrderEvent(ctx, event)
	if err != nil {
		t.Fatalf("expected insert order event to succeed, got %v", err)
	}

	t.Cleanup(func() {
		_, err := store.pool.Exec(ctx, `DELETE FROM order_events WHERE id = $1`, id)
		if err != nil {
			t.Fatalf("expected cleanup order event to succeed, got %v", err)
		}
	})

	var got domain.OrderEvent
	err = store.pool.QueryRow(ctx, `
		SELECT id, order_id, event_type, payload, created_at
		FROM order_events
		WHERE id = $1
	`, id).Scan(
		&got.ID,
		&got.OrderID,
		&got.EventType,
		&got.Payload,
		&got.CreatedAt,
	)
	if err != nil {
		t.Fatalf("expected order event to be found, got %v", err)
	}

	if got.ID != id {
		t.Fatalf("expected id %d, got %d", id, got.ID)
	}

	if got.OrderID != event.OrderID {
		t.Fatalf("expected order id %s, got %s", event.OrderID, got.OrderID)
	}

	if got.EventType != event.EventType {
		t.Fatalf("expected event type %s, got %s", event.EventType, got.EventType)
	}

	assertJSONEqual(t, event.Payload, got.Payload)

	if !got.CreatedAt.Equal(event.CreatedAt) {
		t.Fatalf("expected created_at %v, got %v", event.CreatedAt, got.CreatedAt)
	}
}
