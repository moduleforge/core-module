-- +goose Up

-- Seed: abstract type 'legal_entity', child of 'entity'.
-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM types WHERE slug = 'entity') THEN
    RAISE EXCEPTION 'seed for legal_entity requires parent slug ''entity'' to exist';
  END IF;
END $$;
-- +goose StatementEnd

INSERT INTO types (slug, parent_id, concrete, name, description)
SELECT
  'legal_entity',
  id,
  false,
  'Legal Entity',
  'Abstract type for entities with legal standing (natural persons and corporations).'
FROM types WHERE slug = 'entity';
