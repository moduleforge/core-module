-- name: CreateNaturalPerson :one
INSERT INTO natural_persons (entity_id, given_name, family_name)
VALUES ($1, $2, $3)
RETURNING id, entity_id, given_name, family_name, created_at, updated_at;

-- name: GetNaturalPersonByEntityID :one
SELECT id, entity_id, given_name, family_name, created_at, updated_at
FROM natural_persons
WHERE entity_id = $1;

-- name: UpdateNaturalPerson :exec
UPDATE natural_persons
SET given_name = $2, family_name = $3
WHERE entity_id = $1;
