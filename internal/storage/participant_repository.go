package storage

import(
	"bist-matching-engine/internal/domain"
	"context"
)

func (store *PostgresStore) GetParticipants(ctx context.Context) ([]domain.Participant, error) {
	const query = `
        SELECT
			participants.id AS participant_id,
			participants.name,
			participant_types.name AS type_name
		FROM participants
		JOIN participant_types
		ON participant_types.id = participants.type_id
    `

    rows, err := store.pool.Query(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    participants := make([]domain.Participant, 0)

    for rows.Next() {
        var participant domain.Participant

        err := rows.Scan(
            &participant.ID,
            &participant.Name,
            &participant.Type,
        )
        if err != nil {
            return nil, err
        }

        participants = append(participants,participant)
    }

    if err := rows.Err(); err != nil {
        return nil, err
    }

    return participants, nil
}


func (store *PostgresStore) GetParticipantById(ctx context.Context, id int64) (domain.Participant, error) {
	const query = `
        SELECT
			participants.id AS participant_id,
			participants.name,
			participant_types.name AS type_name
		FROM participants
		JOIN participant_types
		ON participant_types.id = participants.type_id
		WHERE participants.id = $1
    `

	var participant domain.Participant

    err := store.pool.QueryRow(
		ctx,
		query,
		id,
	).Scan(
		&participant.ID,
		&participant.Name,
		&participant.Type,
	)
    

    return participant, err
}
