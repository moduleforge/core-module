-- Seed: abstract type 'legal_entity', child of 'entity'.
INSERT INTO types (slug, parent_id, concrete, name, description)
SELECT
  'legal_entity',
  id,
  false,
  'Legal Entity',
  'Abstract type for entities with legal standing (natural persons and corporations).'
FROM types WHERE slug = 'entity';
