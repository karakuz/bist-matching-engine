CREATE TABLE IF NOT EXISTS market_sessions (
    symbol_code TEXT NOT NULL REFERENCES symbols(code),
    session_date DATE NOT NULL,
    opening_price BIGINT NOT NULL CHECK (opening_price > 0),

    PRIMARY KEY (symbol_code, session_date)
);

INSERT INTO market_sessions(symbol_code, session_date, opening_price) VALUES
('ASELS', CURRENT_DATE, 35000),
('THYAO', CURRENT_DATE, 31000),
('GARAN', CURRENT_DATE, 14000),
('AKBNK', CURRENT_DATE, 6200),
('ISCTR', CURRENT_DATE, 1300)
ON CONFLICT (symbol_code, session_date)
DO UPDATE SET
    opening_price = EXCLUDED.opening_price;





