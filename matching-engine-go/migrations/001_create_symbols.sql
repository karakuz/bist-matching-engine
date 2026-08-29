CREATE TABLE IF NOT EXISTS symbols (
    code TEXT PRIMARY KEY,
    tick_size SMALLINT NOT NULL CHECK (tick_size > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
