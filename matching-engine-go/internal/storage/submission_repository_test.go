package storage

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"bist-matching-engine/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type submissionCommitTx struct {
	pgx.Tx
	commitErr    error
	commitCalled bool
}

func (tx *submissionCommitTx) Commit(context.Context) error {
	tx.commitCalled = true
	return tx.commitErr
}

type unsentCommitError struct{}

func (unsentCommitError) Error() string     { return "commit not sent" }
func (unsentCommitError) SafeToRetry() bool { return true }

func TestCommitSubmissionClassifiesOutcome(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		unknown bool
	}{
		{name: "success"},
		{name: "response lost", err: io.EOF, unknown: true},
		{name: "timeout", err: context.DeadlineExceeded, unknown: true},
		{name: "rollback confirmed", err: pgx.ErrTxCommitRollback},
		{name: "not sent", err: unsentCommitError{}},
		{name: "constraint rejected", err: &pgconn.PgError{Severity: "ERROR", Code: "23503"}},
		{name: "serialization rejected", err: &pgconn.PgError{Severity: "ERROR", Code: "40001"}},
		{name: "server reports unknown completion", err: &pgconn.PgError{Severity: "ERROR", Code: "40003"}, unknown: true},
		{name: "server reports unknown resolution", err: &pgconn.PgError{Severity: "ERROR", Code: "08007"}, unknown: true},
		{name: "server disconnect", err: &pgconn.PgError{Severity: "FATAL", Code: "57P01"}, unknown: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := &submissionCommitTx{commitErr: test.err}
			err := commitSubmission(context.Background(), tx)
			if !tx.commitCalled {
				t.Fatal("Commit was not called")
			}
			if (err == nil) != (test.err == nil) {
				t.Fatalf("unexpected commit result: %v", err)
			}
			if test.err != nil && !errors.Is(err, test.err) {
				t.Fatalf("original error not preserved: %v", err)
			}
			var outcomeErr *CommitOutcomeUnknownError
			if errors.As(err, &outcomeErr) != test.unknown {
				t.Fatalf("unknown=%t, got %v", test.unknown, err)
			}
		})
	}
}

func TestPersistSubmissionRollsBackWhenTradeInsertFails(t *testing.T) {
	store, ctx := newTestStore(t)

	initialization, err := store.GetBookInitialization(ctx, "ASELS")
	if err != nil {
		t.Fatalf("GetBookInitialization failed: %v", err)
	}

	var participantID int64

	err = store.pool.QueryRow(
		ctx,
		`SELECT id FROM participants ORDER BY id LIMIT 1`,
	).Scan(&participantID)
	if err != nil {
		t.Fatalf("get participant: %v", err)
	}

	lastSequence, err := store.GetLastOrderSequence(
		ctx,
		initialization.Symbol.Code,
		initialization.SessionDate,
	)
	if err != nil {
		t.Fatalf("GetLastOrderSequence failed: %v", err)
	}

	order, err := domain.NewOrder(
		uuid.NewString(),
		participantID,
		initialization.Symbol,
		initialization.SessionDate,
		domain.SideBuy,
		initialization.OpeningPrice,
		10,
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("NewOrder failed: %v", err)
	}

	order.Sequence = lastSequence + 1

	if err := store.InsertOrder(ctx, order); err != nil {
		t.Fatalf("InsertOrder failed: %v", err)
	}

	t.Cleanup(func() {
		if _, err := store.pool.Exec(ctx, `DELETE FROM trades WHERE buy_order_id = $1`, order.ID); err != nil {
			t.Errorf("cleanup trades: %v", err)
		}

		if _, err := store.pool.Exec(ctx, `DELETE FROM orders WHERE id = $1`, order.ID); err != nil {
			t.Errorf("cleanup order: %v", err)
		}
	})

	finalOrder := order
	finalOrder.RemainingQuantity = 9
	finalOrder.Status = domain.StatusPartiallyFilled

	duplicateTradeID := uuid.NewString()

	trades := []domain.Trade{
		{
			ID:          duplicateTradeID,
			Symbol:      initialization.Symbol,
			BuyOrderID:  order.ID,
			SellOrderID: "sell-1",
			Price:       order.Price,
			Quantity:    1,
			CreatedAt:   time.Now().UTC(),
		},
		{
			ID:          duplicateTradeID,
			Symbol:      initialization.Symbol,
			BuyOrderID:  order.ID,
			SellOrderID: "sell-2",
			Price:       order.Price,
			Quantity:    1,
			CreatedAt:   time.Now().UTC(),
		},
	}

	err = store.PersistSubmission(
		ctx,
		finalOrder,
		nil,
		trades,
		nil,
	)
	if err == nil {
		t.Fatal("expected transaction failure")
	}

	var storedRemainingQuantity int64
	var storedStatus domain.OrderStatus

	err = store.pool.QueryRow(
		ctx,
		`SELECT remaining_quantity, status FROM orders WHERE id = $1`,
		order.ID,
	).Scan(&storedRemainingQuantity, &storedStatus)
	if err != nil {
		t.Fatalf("query order state: %v", err)
	}

	if storedRemainingQuantity != order.RemainingQuantity {
		t.Fatalf("expected remaining quantity %d after rollback, got %d", order.RemainingQuantity, storedRemainingQuantity)
	}

	if storedStatus != order.Status {
		t.Fatalf("expected status %s after rollback, got %s", order.Status, storedStatus)
	}

	var tradeCount int

	err = store.pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM trades WHERE id = $1`,
		duplicateTradeID,
	).Scan(&tradeCount)
	if err != nil {
		t.Fatalf("query trade count: %v", err)
	}

	if tradeCount != 0 {
		t.Fatalf("expected trade rollback, found %d rows", tradeCount)
	}
}
