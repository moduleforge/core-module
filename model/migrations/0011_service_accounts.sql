CREATE TABLE service_accounts (
  id          BIGSERIAL PRIMARY KEY,
  entity_id   BIGINT NOT NULL UNIQUE REFERENCES entities(id) ON DELETE RESTRICT,
  label       TEXT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Enforce that the entity's fundamental type is exactly 'service_account'.
CREATE FUNCTION service_accounts_check_type() RETURNS TRIGGER AS $$
DECLARE
  v_type_id BIGINT;
BEGIN
  SELECT fundamental_type_id INTO v_type_id FROM entities WHERE id = NEW.entity_id;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'service_accounts: entity_id % does not reference a known entity', NEW.entity_id;
  END IF;
  IF NOT type_is_or_descends_from(v_type_id, 'service_account') THEN
    RAISE EXCEPTION 'service_accounts: entity % fundamental type is not service_account', NEW.entity_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER service_accounts_type_check
  BEFORE INSERT ON service_accounts
  FOR EACH ROW EXECUTE FUNCTION service_accounts_check_type();

CREATE TRIGGER service_accounts_set_updated_at
  BEFORE UPDATE ON service_accounts
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
