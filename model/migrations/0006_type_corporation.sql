-- +goose Up

-- Seed: concrete type 'corporation', child of 'legal_entity'.
-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM types WHERE slug = 'legal_entity') THEN
    RAISE EXCEPTION 'seed for corporation requires parent slug ''legal_entity'' to exist';
  END IF;
END $$;
-- +goose StatementEnd

INSERT INTO types (slug, parent_id, concrete, name, description)
SELECT
  'corporation',
  id,
  true,
  'Corporation',
  'A legal entity organized as a corporation or company.'
FROM types WHERE slug = 'legal_entity';
