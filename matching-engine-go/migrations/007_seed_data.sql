INSERT INTO symbols (code, tick_size) VALUES
    ('ASELS', 10),
    ('THYAO', 10),
    ('GARAN', 10),
    ('AKBNK', 10),
    ('ISCTR', 10)
ON CONFLICT (code) DO UPDATE SET
    tick_size = EXCLUDED.tick_size;

INSERT INTO participant_types (name) VALUES
    ('BROKER'),
    ('TRADER')
ON CONFLICT (name) DO NOTHING;

INSERT INTO participants (name, type_id) VALUES
    ('GARANTII', (SELECT id FROM participant_types WHERE name = 'BROKER')),
    ('DENIIZ',   (SELECT id FROM participant_types WHERE name = 'BROKER')),
    ('BOFFA',    (SELECT id FROM participant_types WHERE name = 'BROKER'))
ON CONFLICT (name) DO NOTHING;

INSERT INTO market_sessions (symbol_code, session_date, opening_price) VALUES
    ('ASELS', CURRENT_DATE, 35000),
    ('THYAO', CURRENT_DATE, 31000),
    ('GARAN', CURRENT_DATE, 14000),
    ('AKBNK', CURRENT_DATE, 6200),
    ('ISCTR', CURRENT_DATE, 1300)
ON CONFLICT (symbol_code, session_date) DO UPDATE SET
    opening_price = EXCLUDED.opening_price;
