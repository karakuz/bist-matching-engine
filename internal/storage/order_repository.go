package storage

import (
	"context"

	"bist-matching-engine/internal/domain"
)

func (store *PostgresStore) InsertOrder(ctx context.Context, order domain.Order) error {
	const query = `
		INSERT INTO orders (
			id,
			participant_id,
			symbol,
			session_date,
			side,
			price,
			quantity,
			remaining_quantity,
			status,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := store.pool.Exec(
		ctx,
		query,
		order.ID,                //$1  id
		order.ParticipantID,     //$2  participant_id
		order.Symbol.Code,       //$3  symbol
		order.SessionDate,       //$4  session_date
		order.Side,              //$5  side
		order.Price,             //$6  price
		order.Quantity,          //$7  quantity
		order.RemainingQuantity, //$8  remaining_quantity
		order.Status,            //$9  status
		order.CreatedAt,         //$10 created_at
	)

	return err
}

func (store *PostgresStore) UpdateOrder(ctx context.Context, order domain.Order) error {
	const query = `
		UPDATE orders
		SET
			remaining_quantity = $2,
			status = $3,
			updated_at = now()
		WHERE id = $1
	`

	_, err := store.pool.Exec(
		ctx,
		query,
		order.ID,
		order.RemainingQuantity,
		order.Status,
	)

	return err
}

func (store *PostgresStore) GetOrderByID(ctx context.Context, id string, symbol domain.Symbol) (domain.Order, error) {
	const query = `
		SELECT
			id,
			participant_id,
			symbol,
			session_date,
			side,
			price,
			quantity,
			remaining_quantity,
			status,
			created_at
		FROM orders
		WHERE id = $1
	`

	var order domain.Order

	err := store.pool.QueryRow(ctx, query, id).Scan(
		&order.ID,
		&order.ParticipantID,
		&order.Symbol.Code,
		&order.SessionDate,
		&order.Side,
		&order.Price,
		&order.Quantity,
		&order.RemainingQuantity,
		&order.Status,
		&order.CreatedAt,
	)
	if err != nil {
		return domain.Order{}, err
	}

	order.Symbol = symbol

	return order, nil
}
