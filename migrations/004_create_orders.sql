CREATE TABLE IF NOT EXISTS orders (
    id TEXT PRIMARY KEY,
    participant_id INTEGER NOT NULL
        REFERENCES participants(id)
        ON DELETE RESTRICT,
    symbol TEXT NOT NULL
        REFERENCES symbols(code)
        ON DELETE RESTRICT,
    session_date DATE NOT NULL,
    CONSTRAINT orders_market_session_fk
        FOREIGN KEY (symbol, session_date)
        REFERENCES market_sessions(symbol_code, session_date)
        ON DELETE RESTRICT,
    side TEXT NOT NULL CHECK (side IN ('BUY', 'SELL')),
    price BIGINT NOT NULL CHECK (price > 0),
    quantity BIGINT NOT NULL CHECK (quantity > 0),
    remaining_quantity BIGINT NOT NULL CHECK (
        remaining_quantity >= 0
        AND remaining_quantity <= quantity
    ),
    status TEXT NOT NULL CHECK (
        status IN ('OPEN', 'PARTIALLY_FILLED', 'FILLED', 'REJECTED')
    ),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_orders_participant_id
ON orders(participant_id);

CREATE INDEX IF NOT EXISTS idx_orders_symbol
ON orders(symbol);
