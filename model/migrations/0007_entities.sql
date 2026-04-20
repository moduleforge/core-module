-- pgcrypto is the single Postgres-specific dependency; provides gen_random_uuid().
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE entities (
  id                    BIGSERIAL PRIMARY KEY,
  uuid                  UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
  fundamental_type_id   BIGINT NOT NULL REFERENCES types(id),
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  archived_at           TIMESTAMPTZ
);

CREATE INDEX entities_fundamental_type_id_idx ON entities(fundamental_type_id);
CREATE INDEX entities_archived_at_idx ON entities(archived_at) WHERE archived_at IS NOT NULL;

-- Enforce that fundamental_type_id must reference a concrete type.
CREATE FUNCTION entities_check_concrete_type() RETURNS TRIGGER AS $$
DECLARE
  v_concrete BOOLEAN;
BEGIN
  SELECT concrete INTO v_concrete FROM types WHERE id = NEW.fundamental_type_id;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'entities: fundamental_type_id % does not reference a known type', NEW.fundamental_type_id;
  END IF;
  IF NOT v_concrete THEN
    RAISE EXCEPTION 'entities: fundamental_type_id must reference a concrete type; % is abstract', NEW.fundamental_type_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Both triggers below are scoped to fundamental_type_id changes only.
-- The concrete-check validates that the (possibly unchanged) new value references a concrete type.
-- The immutable-check raises if the value actually changed (i.e., the column was modified).
CREATE TRIGGER entities_fundamental_type_concrete_check
  BEFORE INSERT OR UPDATE OF fundamental_type_id ON entities
  FOR EACH ROW EXECUTE FUNCTION entities_check_concrete_type();

-- Enforce that fundamental_type_id is immutable after insert.
CREATE FUNCTION entities_immutable_type() RETURNS TRIGGER AS $$
BEGIN
  IF OLD.fundamental_type_id IS DISTINCT FROM NEW.fundamental_type_id THEN
    RAISE EXCEPTION 'entities: fundamental_type_id is immutable after insert';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER entities_fundamental_type_immutable
  BEFORE UPDATE OF fundamental_type_id ON entities
  FOR EACH ROW EXECUTE FUNCTION entities_immutable_type();

CREATE TRIGGER entities_set_updated_at
  BEFORE UPDATE ON entities
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
