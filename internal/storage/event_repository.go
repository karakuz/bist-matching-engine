package storage

import (
	"bist-matching-engine/internal/domain"
	"context"
)

func (store *PostgresStore) InsertOrderEvent(ctx context.Context, event domain.OrderEvent) (int64, error) {
	const query = `
		INSERT INTO order_events (
			order_id,
			event_type,
			payload,
			created_at
		)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`

	var id int64
	err := store.pool.QueryRow(
		ctx,
		query,
		event.OrderID,
		event.EventType,
		event.Payload,
		event.CreatedAt,
	).Scan(&id)

	if err != nil {
		return 0, err
	}

	return id, nil
}