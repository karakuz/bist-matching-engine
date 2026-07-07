package storage

import (
	"bist-matching-engine/internal/domain"
	"context"
	"strings"
	"fmt"
)

/* func (store *PostgresStore) InsertTrades(ctx context.Context, trades []domain.Trade) error {
	if len(trades) == 0 {
		return nil
	}

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	const query = `
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
			query,
			trade.ID,
			trade.Symbol.Code,
			trade.BuyOrderID,
			trade.SellOrderID,
			trade.Price,
			trade.Quantity,
			trade.CreatedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
} */

func (store *PostgresStore) InsertTrades(ctx context.Context, trades []domain.Trade) error {
	if len(trades) == 0 {
		return nil
	}

	var builder strings.Builder
	args := make([]any, 0, len(trades)*7)

	builder.WriteString(`
		INSERT INTO trades (
			id,
			symbol,
			buy_order_id,
			sell_order_id,
			price,
			quantity,
			created_at
		)
		VALUES
	`)

	for i, trade := range trades {
		if i > 0 {
			builder.WriteString(",")
		}

		base := i*7 + 1

		builder.WriteString(fmt.Sprintf(
			" ($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			base,
			base+1,
			base+2,
			base+3,
			base+4,
			base+5,
			base+6,
		))

		args = append(args,
			trade.ID,
			trade.Symbol.Code,
			trade.BuyOrderID,
			trade.SellOrderID,
			trade.Price,
			trade.Quantity,
			trade.CreatedAt,
		)
	}

	_, err := store.pool.Exec(ctx, builder.String(), args...)
	return err
}

func (store *PostgresStore) GetTrade(ctx context.Context, id string) (domain.Trade, error) {
	const query = `
		SELECT 
			id,
			symbol,
			buy_order_id,
			sell_order_id,
			price,
			quantity,
			created_at
		FROM trades
		WHERE id = $1
	`
	var trade domain.Trade

	err := store.pool.QueryRow(
		ctx,
		query,
		id,
	).Scan(
		&trade.ID,
		&trade.Symbol.Code,
		&trade.BuyOrderID,
		&trade.SellOrderID,
		&trade.Price,
		&trade.Quantity,
		&trade.CreatedAt,
	)

	return trade, err
}
