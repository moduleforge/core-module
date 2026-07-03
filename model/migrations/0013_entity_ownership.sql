-- +goose Up

-- 1. Nullable, self-referential ownership column.
--    No ON DELETE clause (default NO ACTION); entities are soft-deleted via
--    archived_at, so there is no hard-DELETE path for the RI action to guard.
ALTER TABLE entities
  ADD COLUMN owner_id BIGINT REFERENCES entities(id);

-- Supports the generic own-arm lookup (e.owner_id = p_actor_entity_id).
CREATE INDEX entities_owner_id_idx ON entities(owner_id) WHERE owner_id IS NOT NULL;

-- 2. "Owns itself" defaulting. Fires BEFORE INSERT only. NEW.id is already
--    populated here: the BIGSERIAL DEFAULT nextval() is evaluated when the
--    candidate tuple is formed, before row-level BEFORE INSERT triggers run.
-- +goose StatementBegin
CREATE FUNCTION entities_owner_default_self() RETURNS TRIGGER AS $$
BEGIN
  IF NEW.owner_id IS NULL
     AND ( type_is_or_descends_from(NEW.fundamental_type_id, 'natural_person')
        OR type_is_or_descends_from(NEW.fundamental_type_id, 'service_account') )
  THEN
    NEW.owner_id := NEW.id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- Trigger NAME is chosen deliberately: 'entities_owner_self_default' sorts
-- alphabetically AFTER the existing 'entities_fundamental_type_concrete_check'
-- ('...f...' < '...o...'), and Postgres fires same-event row triggers in
-- alphabetical name order, so fundamental_type_id is validated first.
CREATE TRIGGER entities_owner_self_default
  BEFORE INSERT ON entities
  FOR EACH ROW EXECUTE FUNCTION entities_owner_default_self();

-- 3. Backfill pre-existing natural_person / service_account rows. This MUST run
--    BEFORE the owner-immutability trigger is created (step 4); otherwise the
--    NULL -> id transition trips that guard. (Corporations / authz groups stay NULL.)
UPDATE entities e
   SET owner_id = e.id
 WHERE e.owner_id IS NULL
   AND ( type_is_or_descends_from(e.fundamental_type_id, 'natural_person')
      OR type_is_or_descends_from(e.fundamental_type_id, 'service_account') );

-- 4. Immutability guard (created AFTER the backfill). BEFORE UPDATE OF owner_id
--    mirrors the existing entities_fundamental_type_immutable pattern (0008):
--    it only fires when owner_id is named in the UPDATE's SET list, and uses
--    IS DISTINCT FROM so a no-op re-set of the same value is allowed.
-- +goose StatementBegin
CREATE FUNCTION entities_immutable_owner() RETURNS TRIGGER AS $$
BEGIN
  IF OLD.owner_id IS DISTINCT FROM NEW.owner_id THEN
    RAISE EXCEPTION 'entities: owner_id is immutable after insert';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER entities_owner_immutable
  BEFORE UPDATE OF owner_id ON entities
  FOR EACH ROW EXECUTE FUNCTION entities_immutable_owner();

-- +goose Down

-- Reverse order: drop the two triggers, then the two functions, then the
-- index, then the column.
DROP TRIGGER IF EXISTS entities_owner_immutable ON entities;
DROP TRIGGER IF EXISTS entities_owner_self_default ON entities;
DROP FUNCTION IF EXISTS entities_immutable_owner();
DROP FUNCTION IF EXISTS entities_owner_default_self();
DROP INDEX IF EXISTS entities_owner_id_idx;
ALTER TABLE entities DROP COLUMN IF EXISTS owner_id;
