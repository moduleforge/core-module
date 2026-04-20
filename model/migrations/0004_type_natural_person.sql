-- Seed: concrete type 'natural_person', child of 'legal_entity'.
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM types WHERE slug = 'legal_entity') THEN
    RAISE EXCEPTION 'seed for natural_person requires parent slug ''legal_entity'' to exist';
  END IF;
END $$;

INSERT INTO types (slug, parent_id, concrete, name, description)
SELECT
  'natural_person',
  id,
  true,
  'Natural Person',
  'A human individual with legal standing.'
FROM types WHERE slug = 'legal_entity';
