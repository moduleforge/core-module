-- name: CreateCorporation :one
INSERT INTO corporations (entity_id, legal_name, jurisdiction, ein)
VALUES ($1, $2, $3, $4)
RETURNING id, entity_id, legal_name, jurisdiction, created_at, updated_at, ein;

-- name: GetCorporationByEntityID :one
SELECT id, entity_id, legal_name, jurisdiction, created_at, updated_at, ein
FROM corporations
WHERE entity_id = $1;

-- NOTE: pass NULL for ein to leave it unchanged; pass an empty bytea
-- to clear it. A non-empty bytea replaces it.
-- name: UpdateCorporation :exec
UPDATE corporations
SET legal_name = $2,
    jurisdiction = $3,
    ein = COALESCE($4, ein)
WHERE entity_id = $1;

-- Re-encrypt-on-read write-back: replaces the stored ein blob with a
-- freshly re-encrypted one. The old-ein predicate is a compare-and-swap
-- guard, not a lookup condition: it ensures the write only lands if the
-- stored blob is still the one that was read, so a concurrent writer's
-- change is never clobbered.
-- name: UpdateCorporationEINBlob :execrows
UPDATE corporations
SET ein = sqlc.arg(new_ein)
WHERE entity_id = sqlc.arg(entity_id) AND ein = sqlc.arg(old_ein);
