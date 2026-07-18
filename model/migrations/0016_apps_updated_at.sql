-- +goose Up

-- apps had no updated_at of its own (0015_apps.sql): the handler surfaced
-- entities.updated_at instead, but that column's trigger only fires on
-- writes to the entities table itself, which UpdateApp never touches. So a
-- slug/name edit never bumped any updated_at the API could see. apps has
-- mutable domain columns (slug, name) — like corporations/service_accounts
-- (0011_corporations.sql/0012_service_accounts.sql) and unlike
-- legal_entities, which has none — so it needs its own updated_at + trigger,
-- mirroring that same pattern.
ALTER TABLE apps ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE TRIGGER apps_set_updated_at
  BEFORE UPDATE ON apps
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down

DROP TRIGGER IF EXISTS apps_set_updated_at ON apps;
ALTER TABLE apps DROP COLUMN IF EXISTS updated_at;
