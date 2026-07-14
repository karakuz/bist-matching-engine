package storage

import(
	"bist-matching-engine/internal/domain"
	"context"
)

type BookInitialization struct {
    Symbol       domain.Symbol
    OpeningPrice int64
}

func (store *PostgresStore) GetBookInitializations(ctx context.Context) ([]BookInitialization, error) {
	const query = `
        SELECT
            symbols.code,
            symbols.tick_size,
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
            &initialization.Symbol.Code,
            &initialization.Symbol.TickSize,
            &initialization.OpeningPrice,
        )
        if err != nil {
            return nil, err
        }

        bookInitializations = append(bookInitializations,initialization)
    }

    if err := rows.Err(); err != nil {
        return nil, err
    }

    return bookInitializations, nil
}

