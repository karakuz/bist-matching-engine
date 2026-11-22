package storage

import (
	"context"
	"errors"
	"fmt"

	"bist-matching-engine/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// CommitOutcomeUnknownError means the commit result could not be confirmed.
// It does not assert that COMMIT reached PostgreSQL or that it succeeded.
type CommitOutcomeUnknownError struct {
	Err error
}

func (err *CommitOutcomeUnknownError) Error() string {
	return fmt.Sprintf("commit submission transaction outcome is unknown: %v", err.Err)
}

func (err *CommitOutcomeUnknownError) Unwrap() error {
	return err.Err
}

func (store *PostgresStore) PersistSubmission(ctx context.Context, incomingOrder domain.Order, restingOrderUpdates []domain.Order, trades []domain.Trade, events []domain.OrderEvent) error {
	tx, err := store.pool.Begin(ctx)
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

	return commitSubmission(ctx, tx)
}

func commitSubmission(ctx context.Context, tx pgx.Tx) error {
	err := tx.Commit(ctx)
	if err == nil {
		return nil
	}

	// pgconn.SafeToRetry means the command was not sent. No retry is made here.
	if errors.Is(err, pgx.ErrTxCommitRollback) || pgconn.SafeToRetry(err) {
		return fmt.Errorf("commit submission transaction: %w", err)
	}
	var postgresErr *pgconn.PgError
	if errors.As(err, &postgresErr) {
		severity := postgresErr.SeverityUnlocalized
		if severity == "" {
			severity = postgresErr.Severity
		}
		// An explicit ERROR response rejects COMMIT, except SQLSTATEs that
		// explicitly report unknown completion or transaction resolution.
		if severity == "ERROR" && postgresErr.Code != "40003" && postgresErr.Code != "08007" {
			return fmt.Errorf("commit submission transaction: %w", err)
		}
	}
	return &CommitOutcomeUnknownError{Err: err}
}
