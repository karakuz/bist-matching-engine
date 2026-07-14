INSERT INTO symbols (code, tick_size) VALUES 
('ASELS', 1),
('THYAO', 1),
('GARAN', 1),
('AKBNK', 1),
('ISCTR', 1)
ON CONFLICT (code) DO UPDATE SET
    tick_size = EXCLUDED.tick_size;

