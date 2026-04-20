-- Seed: concrete type 'service_account', child of 'entity' (not a legal entity).
INSERT INTO types (slug, parent_id, concrete, name, description)
SELECT
  'service_account',
  id,
  true,
  'Service Account',
  'A non-human principal used by automated services and integrations.'
FROM types WHERE slug = 'entity';
