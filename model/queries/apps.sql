-- Entity creation (fundamental_type = 'app') and archival reuse the existing
-- generic entities.sql queries (GetTypeBySlug + CreateEntity to create,
-- ArchiveEntity to archive) — the same pattern every other entity subtype
-- (corporation, natural_person, service_account) already follows. Only the
-- apps-table-specific operations are defined here.

-- name: InsertApp :one
INSERT INTO apps (id, slug, name)
VALUES ($1, $2, $3)
RETURNING id, slug, name;

-- name: GetAppByUUID :one
SELECT
  e.id, e.uuid, a.slug, a.name, e.created_at, e.updated_at, e.archived_at
FROM apps a
JOIN entities e ON a.id = e.id
WHERE e.uuid = $1;

-- name: GetAppBySlug :one
SELECT
  e.id, e.uuid, a.slug, a.name, e.created_at, e.updated_at, e.archived_at
FROM apps a
JOIN entities e ON a.id = e.id
WHERE a.slug = $1;

-- name: ListApps :many
SELECT
  e.id, e.uuid, a.slug, a.name, e.created_at, e.updated_at, e.archived_at
FROM apps a
JOIN entities e ON a.id = e.id
WHERE e.archived_at IS NULL
ORDER BY a.name;

-- name: UpdateApp :exec
UPDATE apps
SET slug = $2, name = $3
WHERE id = $1;
