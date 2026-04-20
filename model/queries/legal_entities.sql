-- name: CreateLegalEntity :one
INSERT INTO legal_entities (entity_id)
VALUES ($1)
RETURNING entity_id;

-- name: GetLegalEntityByEntityID :one
SELECT entity_id
FROM legal_entities
WHERE entity_id = $1;
