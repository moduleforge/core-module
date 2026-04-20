CREATE TABLE types (
  id             BIGSERIAL PRIMARY KEY,
  slug           TEXT UNIQUE NOT NULL,
  parent_id      BIGINT REFERENCES types(id) CHECK (parent_id IS DISTINCT FROM id),
  concrete       BOOLEAN NOT NULL,
  name           TEXT NOT NULL,
  description    TEXT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  deprecated_at  TIMESTAMPTZ
);

CREATE INDEX types_parent_id_idx ON types(parent_id);

-- Append-only enforcement: rows may never be DELETEd,
-- and may only be UPDATEd to set/unset deprecated_at.
CREATE FUNCTION types_reject_mutation() RETURNS TRIGGER AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'types rows are append-only; DELETE is not permitted';
  END IF;
  IF OLD.id IS DISTINCT FROM NEW.id
     OR OLD.slug IS DISTINCT FROM NEW.slug
     OR OLD.parent_id IS DISTINCT FROM NEW.parent_id
     OR OLD.concrete IS DISTINCT FROM NEW.concrete
     OR OLD.name IS DISTINCT FROM NEW.name
     OR OLD.description IS DISTINCT FROM NEW.description
     OR OLD.created_at IS DISTINCT FROM NEW.created_at
  THEN
    RAISE EXCEPTION 'types rows are append-only; only deprecated_at may be updated';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER types_no_delete
  BEFORE DELETE ON types
  FOR EACH ROW EXECUTE FUNCTION types_reject_mutation();

CREATE TRIGGER types_append_only_update
  BEFORE UPDATE ON types
  FOR EACH ROW EXECUTE FUNCTION types_reject_mutation();

-- type_is_or_descends_from(p_type_id, p_target_slug) — returns TRUE if the
-- type identified by p_type_id is, or is a descendant of, the type with slug
-- p_target_slug. Used by subtype BEFORE INSERT triggers to assert ancestry.
-- Placed here (after CREATE TABLE types) so the SQL-language function can
-- resolve the types relation at parse time.
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
