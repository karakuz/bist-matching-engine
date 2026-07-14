package domain

import "testing"

func TestNewParticipant_RejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name            string
		id              int64
		participantName string
		participantType ParticipantType
		wantErr         error
	}{
		{
			name:            "empty id",
			id:              -1,
			participantName: "Demo Broker",
			participantType: ParticipantTypeBroker,
			wantErr:         ErrInvalidParticipantID,
		},
		{
			name:            "empty name",
			id:              1,
			participantName: "   ",
			participantType: ParticipantTypeBroker,
			wantErr:         ErrEmptyParticipantName,
		},
		{
			name:            "invalid type",
			id:              1,
			participantName: "Demo Participant",
			participantType: "UNKNOWN",
			wantErr:         ErrInvalidParticipantType,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewParticipant(
				test.id,
				test.participantName,
				test.participantType,
			)
			if err != test.wantErr {
				t.Fatalf("expected error %v, got %v", test.wantErr, err)
			}
		})
	}
}

func TestNewParticipant_AcceptsSupportedTypes(t *testing.T) {
	tests := []struct {
		name            string
		participantType ParticipantType
	}{
		{name: "broker", participantType: ParticipantTypeBroker},
		{name: "trader", participantType: ParticipantTypeTrader},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			participant, err := NewParticipant(
				1,
				" Demo Participant ",
				test.participantType,
			)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if participant.Name != "Demo Participant" {
				t.Fatalf("unexpected participant name: %q", participant.Name)
			}
			if participant.Type != test.participantType {
				t.Fatalf("expected type %q, got %q", test.participantType, participant.Type)
			}
		})
	}
}
