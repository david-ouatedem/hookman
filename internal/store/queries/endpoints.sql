-- name: CreateEndpoint :exec
INSERT INTO endpoints (id, url, topics, description, signing_secret, enabled, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW());

-- name: GetEndpoint :one
SELECT id, url, topics, description, signing_secret, enabled, created_at, updated_at
FROM endpoints
WHERE id = $1;

-- name: ListEndpoints :many
SELECT id, url, topics, description, signing_secret, enabled, created_at, updated_at
FROM endpoints
ORDER BY created_at DESC;

-- name: UpdateEndpoint :exec
UPDATE endpoints
SET
  url = COALESCE(sqlc.narg('url'), url),
  topics = COALESCE(sqlc.narg('topics'), topics),
  description = COALESCE(sqlc.narg('description'), description),
  enabled = COALESCE(sqlc.narg('enabled'), enabled),
  updated_at = NOW()
WHERE id = sqlc.arg('id');

-- name: DeleteEndpoint :exec
DELETE FROM endpoints WHERE id = $1;

-- name: GetEndpointsByTopic :many
SELECT id, url, topics, description, signing_secret, enabled, created_at, updated_at
FROM endpoints
WHERE enabled = TRUE AND $1 = ANY(topics);
