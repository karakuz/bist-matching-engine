package storage

import (
	"context"

	"bist-matching-engine/internal/domain"
)

func (store *PostgresStore) InsertSymbol(ctx context.Context, symbol domain.Symbol) error {
	const query = `
		INSERT INTO symbols (
			code,
			tick_size,
			created_at
		)
		VALUES ($1, $2, now())
	`

	_, err := store.pool.Exec(
		ctx,
		query,
		symbol.Code,
		symbol.TickSize,
	)

	return err
}

func (store *PostgresStore) GetSymbol(ctx context.Context, code string) (domain.Symbol, error) {
	const query = `
		SELECT 
			code, 
			tick_size
		FROM symbols
		WHERE code = $1
	`
	var symbol domain.Symbol

	err := store.pool.QueryRow(
		ctx,
		query,
		code,
	).Scan(&symbol.Code, &symbol.TickSize)

	return symbol, err
}
