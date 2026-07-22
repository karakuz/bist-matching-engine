package storage

import (
	"context"
	"time"
	"strings"
	"fmt"

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

func (store *PostgresStore) GetRestingOrdersForSession(
	ctx context.Context,
	symbol domain.Symbol,
	sessionDate time.Time,
) ([]domain.Order, error) {
	const query = `
		SELECT
			id,
			participant_id,
			side,
			price,
			quantity,
			remaining_quantity,
			status,
			created_at
		FROM orders
		WHERE symbol = $1
			AND session_date = $2
			AND status IN ('OPEN', 'PARTIALLY_FILLED')
		ORDER BY created_at ASC, id ASC
	`

	rows, err := store.pool.Query(ctx, query, symbol.Code, sessionDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]domain.Order, 0)
	for rows.Next() {
		var order domain.Order
		if err := rows.Scan(
			&order.ID,
			&order.ParticipantID,
			&order.Side,
			&order.Price,
			&order.Quantity,
			&order.RemainingQuantity,
			&order.Status,
			&order.CreatedAt,
		); err != nil {
			return nil, err
		}

		order.Symbol = symbol
		order.SessionDate = sessionDate
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

/* 
This produces one statement such as:
	UPDATE orders
	SET ...
	FROM (
		VALUES
			('incoming-id', 9, 'PARTIALLY_FILLED'),
			('resting-id-1', 0, 'FILLED'),
			('resting-id-2', 40, 'PARTIALLY_FILLED')
	) ...
*/
func (store *PostgresStore) UpdateOrders(
	ctx context.Context,
	orders []domain.Order,
) error {
	if len(orders) == 0 {
		return nil
	}

	var builder strings.Builder
	args := make([]any, 0, len(orders)*3)
	seenOrderIDs := make(map[string]struct{}, len(orders))

	builder.WriteString(`
		UPDATE orders AS stored_order
		SET
			remaining_quantity = updated_order.remaining_quantity,
			status = updated_order.status,
			updated_at = now()
		FROM (
			VALUES
	`)

	for orderIndex, order := range orders {
		if _, exists := seenOrderIDs[order.ID]; exists {
			return fmt.Errorf(
				"duplicate order ID in update: %s",
				order.ID,
			)
		}
		seenOrderIDs[order.ID] = struct{}{}

		if orderIndex > 0 {
			builder.WriteString(",")
		}

		base := orderIndex*3 + 1

		builder.WriteString(fmt.Sprintf(
			"($%d::text, $%d::bigint, $%d::text)",
			base,
			base+1,
			base+2,
		))

		args = append(
			args,
			order.ID,
			order.RemainingQuantity,
			order.Status,
		)
	}

	builder.WriteString(`
		) AS updated_order (
			id,
			remaining_quantity,
			status
		)
		WHERE stored_order.id = updated_order.id
	`)

	commandTag, err := store.pool.Exec(
		ctx,
		builder.String(),
		args...,
	)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() != int64(len(orders)) {
		return fmt.Errorf(
			"expected to update %d orders, updated %d",
			len(orders),
			commandTag.RowsAffected(),
		)
	}

	return nil
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
