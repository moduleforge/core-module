-- name: GetTypeBySlug :one
SELECT id, slug, parent_id, concrete, name, description, created_at, deprecated_at
FROM types
WHERE slug = $1;

-- name: GetTypeByID :one
SELECT id, slug, parent_id, concrete, name, description, created_at, deprecated_at
FROM types
WHERE id = $1;
