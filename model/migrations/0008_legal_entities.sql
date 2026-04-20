-- legal_entities is a pure FK-anchor table: no kind, no display_name, no
-- synthetic id, no timestamps (those live on entities). Any entity whose
-- fundamental type descends from 'legal_entity' may have a row here.
CREATE TABLE legal_entities (
  entity_id BIGINT PRIMARY KEY REFERENCES entities(id) ON DELETE RESTRICT
);

-- Enforce that only entities whose fundamental type descends from 'legal_entity'
-- may be inserted into this table.
CREATE FUNCTION legal_entities_check_type() RETURNS TRIGGER AS $$
DECLARE
  v_type_id BIGINT;
BEGIN
  SELECT fundamental_type_id INTO v_type_id FROM entities WHERE id = NEW.entity_id;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'legal_entities: entity_id % does not reference a known entity', NEW.entity_id;
  END IF;
  IF NOT type_is_or_descends_from(v_type_id, 'legal_entity') THEN
    RAISE EXCEPTION 'legal_entities: entity % fundamental type does not descend from legal_entity', NEW.entity_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER legal_entities_type_check
  BEFORE INSERT ON legal_entities
  FOR EACH ROW EXECUTE FUNCTION legal_entities_check_type();
