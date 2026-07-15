WITH seed_orders AS (
    SELECT
        order_number,
        'BUY'::TEXT AS side,
        35000 - (50 * (((order_number - 1) % 6) + 1)) AS price,
        CURRENT_DATE + TIME '09:30:00'
            + (order_number * INTERVAL '1 second') AS created_at
    FROM generate_series(1, 26) AS order_number

    UNION ALL

    SELECT
        order_number,
        'SELL'::TEXT AS side,
        35000 + (50 * (((order_number - 1) % 6) + 1)) AS price,
        CURRENT_DATE + TIME '09:30:00'
            + ((order_number + 26) * INTERVAL '1 second') AS created_at
    FROM generate_series(1, 26) AS order_number
),
orders_with_participants AS (
    SELECT
        seed_orders.*,
        participants.id AS participant_id,
        50 + (((seed_orders.order_number - 1) % 5) * 25) AS quantity
    FROM seed_orders
    JOIN participants
        ON participants.name = (
            ARRAY['GARANTII', 'DENIIZ', 'BOFFA']
        )[((seed_orders.order_number - 1) % 3) + 1]
)
INSERT INTO orders (
    id,
    participant_id,
    symbol,
    session_date,
    side,
    price,
    quantity,
    remaining_quantity,
    status,
    created_at,
    updated_at
)
SELECT
    'seed-asels-'
        || to_char(CURRENT_DATE, 'YYYYMMDD')
        || '-'
        || lower(side)
        || '-'
        || lpad(order_number::TEXT, 2, '0'),
    participant_id,
    'ASELS',
    CURRENT_DATE,
    side,
    price,
    quantity,
    quantity,
    'OPEN',
    created_at,
    created_at
FROM orders_with_participants
ON CONFLICT (id) DO NOTHING;
