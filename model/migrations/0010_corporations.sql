CREATE TABLE corporations (
  id           BIGSERIAL PRIMARY KEY,
  entity_id    BIGINT NOT NULL UNIQUE REFERENCES legal_entities(entity_id) ON DELETE RESTRICT,
  legal_name   TEXT NOT NULL,
  jurisdiction TEXT,
  -- legal_id intentionally omitted in v1
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Consumers should prefer joining through entity_id rather than the synthetic id.

-- Enforce that the entity's fundamental type is exactly 'corporation'.
CREATE FUNCTION corporations_check_type() RETURNS TRIGGER AS $$
DECLARE
  v_type_id BIGINT;
BEGIN
  SELECT fundamental_type_id INTO v_type_id FROM entities WHERE id = NEW.entity_id;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'corporations: entity_id % does not reference a known entity', NEW.entity_id;
  END IF;
  IF NOT type_is_or_descends_from(v_type_id, 'corporation') THEN
    RAISE EXCEPTION 'corporations: entity % fundamental type is not corporation', NEW.entity_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER corporations_type_check
  BEFORE INSERT ON corporations
  FOR EACH ROW EXECUTE FUNCTION corporations_check_type();

CREATE TRIGGER corporations_set_updated_at
  BEFORE UPDATE ON corporations
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
