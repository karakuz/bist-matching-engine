CREATE TABLE IF NOT EXISTS trades (
    id TEXT PRIMARY KEY,
    symbol TEXT NOT NULL
        REFERENCES symbols(code)
        ON DELETE RESTRICT,
    buy_order_id TEXT NOT NULL,
    sell_order_id TEXT NOT NULL,
    price BIGINT NOT NULL CHECK (price > 0),
    quantity BIGINT NOT NULL CHECK (quantity > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_trades_symbol
ON trades(symbol);
