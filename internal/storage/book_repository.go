package storage

import (
	"bist-matching-engine/internal/domain"
	"context"
	"errors"
	"time"
	"fmt"
)

var (
	ErrBookInitializationNotFound = errors.New("BookInitialization not found")
)

type BookInitialization struct {
	Symbol       domain.Symbol
	SessionDate  time.Time
	OpeningPrice int64
}

func (store *PostgresStore) GetBookInitializations(ctx context.Context) ([]BookInitialization, error) {
	const query = `
        SELECT
            symbols.code,
            symbols.tick_size,
            market_sessions.session_date,
            market_sessions.opening_price
        FROM symbols
        JOIN market_sessions
		ON market_sessions.symbol_code = symbols.code
        WHERE market_sessions.session_date = (
			SELECT MAX(session_date)
			FROM market_sessions
		)
        ORDER BY symbols.code
    `

	rows, err := store.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bookInitializations := make([]BookInitialization, 0)

	for rows.Next() {
		var initialization BookInitialization

		err := rows.Scan(
			&initialization.Symbol.Code,     //symbols.code
			&initialization.Symbol.TickSize, //symbols.tick_size
			&initialization.SessionDate,     //market_sessions.session_date
			&initialization.OpeningPrice,    //market_sessions.opening_price
		)
		if err != nil {
			return nil, err
		}

		bookInitializations = append(bookInitializations, initialization)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return bookInitializations, nil
}

func (store *PostgresStore) GetBookInitialization(ctx context.Context, symbolCode string) (BookInitialization, error) {
	initializations, err := store.GetBookInitializations(ctx)
	if err != nil {
		return BookInitialization{}, err
	}

	for _, initialization := range initializations {
		if initialization.Symbol.Code == symbolCode {
			return initialization, nil
		}
	}

	return BookInitialization{}, fmt.Errorf(
		"%w for symbol %q",
		ErrBookInitializationNotFound,
		symbolCode,
	)
}
