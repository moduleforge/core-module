-- +goose Up

-- apps is a pure FK-anchor table, keyed on entities.id (per the change
-- request: apps.id becomes a FK to entities.id, not a separate surrogate
-- key). No uuid, no timestamps, no archived_at: those all live on entities
-- (0008_entities.sql). The app's UUID is entities.uuid; archival is
-- entities.archived_at.
CREATE TABLE apps (
  id    BIGINT PRIMARY KEY REFERENCES entities(id) ON DELETE RESTRICT,
  slug  TEXT NOT NULL UNIQUE,
  name  TEXT NOT NULL
);

-- Enforce that only entities whose fundamental type descends from 'app'
-- may be inserted into this table.
-- +goose StatementBegin
CREATE FUNCTION apps_check_type() RETURNS TRIGGER AS $$
DECLARE
  v_type_id BIGINT;
BEGIN
  SELECT fundamental_type_id INTO v_type_id FROM entities WHERE id = NEW.id;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'apps: id % does not reference a known entity', NEW.id;
  END IF;
  IF NOT type_is_or_descends_from(v_type_id, 'app') THEN
    RAISE EXCEPTION 'apps: entity % fundamental type does not descend from app', NEW.id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER apps_type_check
  BEFORE INSERT ON apps
  FOR EACH ROW EXECUTE FUNCTION apps_check_type();

-- +goose Down

DROP TRIGGER IF EXISTS apps_type_check ON apps;
DROP FUNCTION IF EXISTS apps_check_type();
DROP TABLE IF EXISTS apps;
