package storage

import (
	"testing"
)

func TestPostgresStore_GetParticipantById(t *testing.T) {
	store, ctx := newTestStore(t)

	participant, err := store.GetParticipantById(ctx, 1)
	if err != nil {
		t.Fatalf("got error on GetParticipantById: %v", err)
	}
	if participant.Name != "GARANTII" {
		t.Fatalf("expected participant name: GARANTII, got %s", participant.Name)
	}
	if participant.Type != "BROKER" {
		t.Fatalf("expected participant name: BROKER, got %s", participant.Type)
	}
}

func TestPostgresStore_GetAllParticipants(t *testing.T) {
	store, ctx := newTestStore(t)

	participants, err := store.GetParticipants(ctx)
	if err != nil {
		t.Fatalf("got error on GetParticipantById: %v", err)
	}
	if len(participants) != 3 {
		t.Fatalf("expected 3 participants, got %d", len(participants))
	}
}
