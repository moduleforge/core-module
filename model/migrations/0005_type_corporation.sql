-- Seed: concrete type 'corporation', child of 'legal_entity'.
INSERT INTO types (slug, parent_id, concrete, name, description)
SELECT
  'corporation',
  id,
  true,
  'Corporation',
  'A legal entity organized as a corporation or company.'
FROM types WHERE slug = 'legal_entity';
