ALTER TABLE orders
ADD COLUMN IF NOT EXISTS participant_id INTEGER NOT NULL
REFERENCES participants(id)
ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_orders_participant_id
ON orders(participant_id);