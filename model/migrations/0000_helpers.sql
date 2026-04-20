-- Helper functions used across migrations.

-- set_updated_at() — generic trigger function that stamps updated_at = now()
-- on any table that has an updated_at column. Applied via BEFORE UPDATE triggers.
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- type_is_or_descends_from(p_type_id, p_target_slug) — returns TRUE if the
-- type identified by p_type_id is, or is a descendant of, the type with slug
-- p_target_slug. Used by subtype BEFORE INSERT triggers to assert ancestry.
CREATE OR REPLACE FUNCTION type_is_or_descends_from(p_type_id BIGINT, p_target_slug TEXT)
RETURNS BOOLEAN AS $$
WITH RECURSIVE walk AS (
  SELECT id, slug, parent_id FROM types WHERE id = p_type_id
  UNION ALL
  SELECT t.id, t.slug, t.parent_id
  FROM types t JOIN walk w ON t.id = w.parent_id
)
SELECT EXISTS (SELECT 1 FROM walk WHERE slug = p_target_slug);
$$ LANGUAGE sql STABLE;
