-- Seed: concrete type 'service_account', child of 'entity' (not a legal entity).
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM types WHERE slug = 'entity') THEN
    RAISE EXCEPTION 'seed for service_account requires parent slug ''entity'' to exist';
  END IF;
END $$;

INSERT INTO types (slug, parent_id, concrete, name, description)
SELECT
  'service_account',
  id,
  true,
  'Service Account',
  'A non-human principal used by automated services and integrations.'
FROM types WHERE slug = 'entity';
