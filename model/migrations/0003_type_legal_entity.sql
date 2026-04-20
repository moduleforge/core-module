-- Seed: abstract type 'legal_entity', child of 'entity'.
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM types WHERE slug = 'entity') THEN
    RAISE EXCEPTION 'seed for legal_entity requires parent slug ''entity'' to exist';
  END IF;
END $$;

INSERT INTO types (slug, parent_id, concrete, name, description)
SELECT
  'legal_entity',
  id,
  false,
  'Legal Entity',
  'Abstract type for entities with legal standing (natural persons and corporations).'
FROM types WHERE slug = 'entity';
