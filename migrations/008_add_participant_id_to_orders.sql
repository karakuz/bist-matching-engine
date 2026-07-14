ALTER TABLE orders
ADD COLUMN participant_id INTEGER NOT NULL
REFERENCES participants(id)
ON DELETE RESTRICT;

CREATE INDEX idx_orders_participant_id
ON orders(participant_id);