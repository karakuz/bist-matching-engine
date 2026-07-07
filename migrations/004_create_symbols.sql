CREATE TABLE IF NOT EXISTS symbols (
    code TEXT PRIMARY KEY,
    tick_size SMALLINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
