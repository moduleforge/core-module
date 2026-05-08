-- +goose Up
--
-- Access function stubs for core-module entity resources.
--
-- These define the function signatures that list/search queries JOIN against
-- for row-level scoping. The bodies here are stubs that return the empty set.
-- At application startup, the chosen Authorizer implementation calls
-- core-module/api/authz/setup.ApplyFuncs(...) which replaces these bodies via
-- CREATE OR REPLACE FUNCTION with the real policy.
--
-- See core-module/docs/architecture/authorization-design.md "Row-level scoping".

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION accessible_natural_person_ids_for_actor(p_actor_entity_id BIGINT)
RETURNS TABLE(entity_id BIGINT) LANGUAGE sql STABLE AS $$
    SELECT 0::BIGINT AS entity_id WHERE FALSE
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION accessible_corporation_ids_for_actor(p_actor_entity_id BIGINT)
RETURNS TABLE(entity_id BIGINT) LANGUAGE sql STABLE AS $$
    SELECT 0::BIGINT AS entity_id WHERE FALSE
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION accessible_service_account_ids_for_actor(p_actor_entity_id BIGINT)
RETURNS TABLE(entity_id BIGINT) LANGUAGE sql STABLE AS $$
    SELECT 0::BIGINT AS entity_id WHERE FALSE
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION accessible_legal_entity_ids_for_actor(p_actor_entity_id BIGINT)
RETURNS TABLE(entity_id BIGINT) LANGUAGE sql STABLE AS $$
    SELECT 0::BIGINT AS entity_id WHERE FALSE
$$;
-- +goose StatementEnd

-- +goose Down

DROP FUNCTION IF EXISTS accessible_legal_entity_ids_for_actor(BIGINT);
DROP FUNCTION IF EXISTS accessible_service_account_ids_for_actor(BIGINT);
DROP FUNCTION IF EXISTS accessible_corporation_ids_for_actor(BIGINT);
DROP FUNCTION IF EXISTS accessible_natural_person_ids_for_actor(BIGINT);
