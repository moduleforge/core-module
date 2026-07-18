-- +goose Up

-- Seed: concrete type 'app', child of 'entity' (not a legal entity).
-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM types WHERE slug = 'entity') THEN
    RAISE EXCEPTION 'seed for app requires parent slug ''entity'' to exist';
  END IF;
END $$;
-- +goose StatementEnd

INSERT INTO types (slug, parent_id, concrete, name, description)
SELECT
  'app',
  id,
  true,
  'App',
  'A registered application principal that consumes moduleforge module APIs.'
FROM types WHERE slug = 'entity';
