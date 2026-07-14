CREATE TABLE IF NOT EXISTS participant_types (
    id SERIAL PRIMARY KEY,
    name VARCHAR(30) NOT NULL,
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (name)
);

INSERT INTO participant_types("name") VALUES
('BROKER'),
('TRADER')
ON CONFLICT ("name") DO NOTHING;

CREATE TABLE IF NOT EXISTS participants (
    id SERIAL PRIMARY KEY,
    "name" VARCHAR(30) NOT NULL,
    "type_id" INTEGER NOT NULL REFERENCES participant_types(id),
    created_on TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE ("name")
);

INSERT INTO participants (name, type_id)
VALUES
    ('GARANTII', (SELECT id FROM participant_types WHERE name = 'BROKER')),
    ('DENIIZ',   (SELECT id FROM participant_types WHERE name = 'BROKER')),
    ('BOFFA',    (SELECT id FROM participant_types WHERE name = 'BROKER'))
ON CONFLICT (name) DO NOTHING;