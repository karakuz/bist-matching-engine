CREATE TABLE IF NOT EXISTS market_sessions (
    symbol_code TEXT NOT NULL
        REFERENCES symbols(code)
        ON DELETE RESTRICT,
    session_date DATE NOT NULL,
    opening_price BIGINT NOT NULL CHECK (opening_price > 0),
    PRIMARY KEY (symbol_code, session_date)
);
