package storage

import (
	"context"

	"bist-matching-engine/internal/domain"
)

func (store *PostgresStore) InsertOrder(ctx context.Context, order domain.Order) error {
	const query = `
		INSERT INTO orders (
			id,
			symbol,
			side,
			price,
			quantity,
			remaining_quantity,
			status,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
	`

	_, err := store.pool.Exec(
		ctx,
		query,
		order.ID,
		order.Symbol.Code,
		order.Side,
		order.Price,
		order.Quantity,
		order.RemainingQuantity,
		order.Status,
		order.CreatedAt,
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
