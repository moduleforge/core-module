-- +goose Up

CREATE TABLE natural_persons (
  id          BIGSERIAL PRIMARY KEY,
  entity_id   BIGINT NOT NULL UNIQUE REFERENCES legal_entities(entity_id) ON DELETE RESTRICT,
  given_name  TEXT,
  family_name TEXT,
  -- ssn holds an AES-256-GCM blob (version || nonce || ciphertext || tag)
  -- produced by the application. NULL means no SSN is recorded. The
  -- database never sees plaintext.
  ssn         BYTEA,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Consumers should prefer joining through entity_id rather than the synthetic id.

-- Enforce that the entity's fundamental type is exactly 'natural_person'.
-- +goose StatementBegin
CREATE FUNCTION natural_persons_check_type() RETURNS TRIGGER AS $$
DECLARE
  v_type_id BIGINT;
BEGIN
  SELECT fundamental_type_id INTO v_type_id FROM entities WHERE id = NEW.entity_id;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'natural_persons: entity_id % does not reference a known entity', NEW.entity_id;
  END IF;
  IF NOT type_is_or_descends_from(v_type_id, 'natural_person') THEN
    RAISE EXCEPTION 'natural_persons: entity % fundamental type is not natural_person', NEW.entity_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER natural_persons_type_check
  BEFORE INSERT ON natural_persons
  FOR EACH ROW EXECUTE FUNCTION natural_persons_check_type();

CREATE TRIGGER natural_persons_set_updated_at
  BEFORE UPDATE ON natural_persons
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
