-- name: CreateEntity :one
INSERT INTO entities (fundamental_type_id)
VALUES ($1)
RETURNING id, uuid, fundamental_type_id, created_at, updated_at, archived_at, owner_id;

-- name: CreateEntityWithOwner :one
INSERT INTO entities (fundamental_type_id, owner_id)
VALUES ($1, $2)
RETURNING id, uuid, fundamental_type_id, created_at, updated_at, archived_at, owner_id;

-- name: GetEntityByUUID :one
SELECT
  e.id, e.uuid, e.fundamental_type_id,
  t.slug AS fundamental_type_slug,
  e.created_at, e.updated_at, e.archived_at
FROM entities e
JOIN types t ON e.fundamental_type_id = t.id
WHERE e.uuid = $1;

-- name: GetEntityByID :one
SELECT
  e.id, e.uuid, e.fundamental_type_id,
  t.slug AS fundamental_type_slug,
  e.created_at, e.updated_at, e.archived_at
FROM entities e
JOIN types t ON e.fundamental_type_id = t.id
WHERE e.id = $1;

-- name: ArchiveEntity :exec
UPDATE entities
SET archived_at = now()
WHERE uuid = $1 AND archived_at IS NULL;

-- name: UnarchiveEntity :exec
UPDATE entities
SET archived_at = NULL
WHERE uuid = $1 AND archived_at IS NOT NULL;
