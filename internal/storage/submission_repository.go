package storage

import (
	"context"
	"fmt"

	"bist-matching-engine/internal/domain"
)

func (store *PostgresStore) PersistSubmission(ctx context.Context, incomingOrder domain.Order, restingOrderUpdates []domain.Order, trades []domain.Trade, events []domain.OrderEvent) error {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin submission transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const insertOrderQuery = `
		INSERT INTO orders (
			id,
			sequence_number,
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
		VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11
		)
	`

	_, err = tx.Exec(
		ctx,
		insertOrderQuery,
		incomingOrder.ID,
		incomingOrder.Sequence,
		incomingOrder.ParticipantID,
		incomingOrder.Symbol.Code,
		incomingOrder.SessionDate,
		incomingOrder.Side,
		incomingOrder.Price,
		incomingOrder.Quantity,
		incomingOrder.RemainingQuantity,
		incomingOrder.Status,
		incomingOrder.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert incoming order: %w", err)
	}

	const updateOrderQuery = `
		UPDATE orders
		SET
			remaining_quantity = $2,
			status = $3,
			updated_at = now()
		WHERE id = $1
	`

	for _, order := range restingOrderUpdates {
		commandTag, err := tx.Exec(
			ctx,
			updateOrderQuery,
			order.ID,
			order.RemainingQuantity,
			order.Status,
		)
		if err != nil {
			return fmt.Errorf("update resting order %s: %w", order.ID, err)
		}

		if commandTag.RowsAffected() != 1 {
			return fmt.Errorf("expected to update resting order %s", order.ID)
		}
	}

	const insertTradeQuery = `
		INSERT INTO trades (
			id,
			symbol,
			buy_order_id,
			sell_order_id,
			price,
			quantity,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	for _, trade := range trades {
		_, err := tx.Exec(
			ctx,
			insertTradeQuery,
			trade.ID,
			trade.Symbol.Code,
			trade.BuyOrderID,
			trade.SellOrderID,
			trade.Price,
			trade.Quantity,
			trade.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert trade %s: %w", trade.ID, err)
		}
	}

	const insertEventQuery = `
		INSERT INTO order_events (
			order_id,
			event_type,
			payload,
			created_at
		)
		VALUES ($1, $2, $3, $4)
	`

	for _, event := range events {
		_, err := tx.Exec(
			ctx,
			insertEventQuery,
			event.OrderID,
			event.EventType,
			event.Payload,
			event.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert order event: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit submission transaction: %w", err)
	}

	return nil
}