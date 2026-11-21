package app

import (
	"context"
	"fmt"

	"bist-matching-engine/internal/domain"
)

type ParticipantReader interface {
	GetParticipants(context.Context) ([]domain.Participant, error)
}

func BuildParticipantIdNameMapping(
	ctx context.Context,
	reader ParticipantReader,
) (map[int64]string, error) {
	participants, err := reader.GetParticipants(ctx)
	if err != nil {
		return nil, fmt.Errorf("get participants: %w", err)
	}

	mapping := make(map[int64]string, len(participants))
	for _, participant := range participants {
		mapping[participant.ID] = participant.Name
	}

	return mapping, nil
}