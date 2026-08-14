-- name: CreateNaturalPerson :one
INSERT INTO natural_persons (entity_id, given_name, family_name, ssn)
VALUES ($1, $2, $3, $4)
RETURNING id, entity_id, given_name, family_name, created_at, updated_at, ssn;

-- name: GetNaturalPersonByEntityID :one
SELECT id, entity_id, given_name, family_name, created_at, updated_at, ssn
FROM natural_persons
WHERE entity_id = $1;

-- NOTE: pass NULL for ssn to leave it unchanged; pass an empty bytea
-- ('\x'::bytea / []byte{}) to clear it. A non-empty bytea replaces it.
-- name: UpdateNaturalPerson :exec
UPDATE natural_persons
SET given_name = $2,
    family_name = $3,
    ssn = COALESCE($4, ssn)
WHERE entity_id = $1;

-- Re-encrypt-on-read write-back: replaces the stored ssn blob with a
-- freshly re-encrypted one. The old-ssn predicate is a compare-and-swap
-- guard, not a lookup condition: it ensures the write only lands if the
-- stored blob is still the one that was read, so a concurrent writer's
-- change is never clobbered.
-- name: UpdateNaturalPersonSSNBlob :execrows
UPDATE natural_persons
SET ssn = sqlc.arg(new_ssn)
WHERE entity_id = sqlc.arg(entity_id) AND ssn = sqlc.arg(old_ssn);
