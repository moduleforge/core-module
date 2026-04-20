-- Seed: concrete type 'natural_person', child of 'legal_entity'.
INSERT INTO types (slug, parent_id, concrete, name, description)
SELECT
  'natural_person',
  id,
  true,
  'Natural Person',
  'A human individual with legal standing.'
FROM types WHERE slug = 'legal_entity';
