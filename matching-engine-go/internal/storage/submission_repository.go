package storage

import (
	"context"
	"fmt"
	"log"
	"time"

	"bist-matching-engine/internal/domain"

	"github.com/jackc/pgx/v5"
)

func (store *PostgresStore) PersistSubmission(ctx context.Context, incomingOrder domain.Order, restingOrderUpdates []domain.Order, trades []domain.Trade, events []domain.OrderEvent) error {
	stageStarted := time.Now()
	tx, err := store.pool.Begin(ctx)
	log.Printf("order=%s symbol=%s stage=db_begin duration=%s err=%v", incomingOrder.ID, incomingOrder.Symbol.Code, time.Since(stageStarted), err)
	if err != nil {
		return fmt.Errorf("begin submission transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	ordersToUpdate := []domain.Order{incomingOrder}
	ordersToUpdate = append(ordersToUpdate, restingOrderUpdates...)
	batch := &pgx.Batch{}

	const updateOrderQuery = `
		UPDATE orders
		SET
			remaining_quantity = $2,
			status = $3,
			updated_at = now()
		WHERE id = $1
	`

	for _, order := range ordersToUpdate {
		batch.Queue(
			updateOrderQuery,
			order.ID,
			order.RemainingQuantity,
			order.Status,
		)
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
		batch.Queue(
			insertTradeQuery,
			trade.ID,
			trade.Symbol.Code,
			trade.BuyOrderID,
			trade.SellOrderID,
			trade.Price,
			trade.Quantity,
			trade.CreatedAt,
		)
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
		batch.Queue(
			insertEventQuery,
			event.OrderID,
			event.EventType,
			event.Payload,
			event.CreatedAt,
		)
	}

	stageStarted = time.Now()
	results := tx.SendBatch(ctx, batch)
	// Close results before rollback on any error, and explicitly before commit.
	defer results.Close()

	for _, order := range ordersToUpdate {
		commandTag, err := results.Exec()
		if err != nil {
			return fmt.Errorf("update order %s: %w", order.ID, err)
		}
		if commandTag.RowsAffected() != 1 {
			return fmt.Errorf("expected to update order %s", order.ID)
		}
	}
	for _, trade := range trades {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("insert trade %s: %w", trade.ID, err)
		}
	}
	for _, event := range events {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("insert order event for %s: %w", event.OrderID, err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("close submission batch: %w", err)
	}
	log.Printf("order=%s symbol=%s stage=db_batch duration=%s orders=%d trades=%d events=%d", incomingOrder.ID, incomingOrder.Symbol.Code, time.Since(stageStarted), len(ordersToUpdate), len(trades), len(events))

	stageStarted = time.Now()
	err = tx.Commit(ctx)
	log.Printf("order=%s symbol=%s stage=db_commit duration=%s err=%v", incomingOrder.ID, incomingOrder.Symbol.Code, time.Since(stageStarted), err)
	if err != nil {
		return fmt.Errorf("commit submission transaction: %w", err)
	}

	return nil
}
