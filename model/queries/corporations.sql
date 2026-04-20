-- name: CreateCorporation :one
INSERT INTO corporations (entity_id, legal_name, jurisdiction)
VALUES ($1, $2, $3)
RETURNING id, entity_id, legal_name, jurisdiction, created_at, updated_at;

-- name: GetCorporationByEntityID :one
SELECT id, entity_id, legal_name, jurisdiction, created_at, updated_at
FROM corporations
WHERE entity_id = $1;

-- name: UpdateCorporation :exec
UPDATE corporations
SET legal_name = $2, jurisdiction = $3
WHERE entity_id = $1;
