-- name: CreateEvent :exec
INSERT INTO events (id, topic, payload, idempotency_key, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW(), NOW());

-- name: GetEvent :one
SELECT id, topic, payload, idempotency_key, status, created_at, updated_at
FROM events
WHERE id = $1;

-- name: ListEvents :many
SELECT id, topic, payload, idempotency_key, status, created_at, updated_at
FROM events
WHERE
  (sqlc.narg('topic')::text IS NULL OR topic = sqlc.narg('topic')) AND
  (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')) AND
  (sqlc.narg('before_id')::text IS NULL OR id < sqlc.narg('before_id'))
ORDER BY id DESC
LIMIT sqlc.arg('limit_count');

-- name: UpdateEventStatus :exec
UPDATE events SET status = $2, updated_at = NOW() WHERE id = $1;

-- name: GetPendingEvents :many
SELECT id, topic, payload, idempotency_key, status, created_at, updated_at
FROM events
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: GetEventByIdempotencyKey :one
SELECT id, topic, payload, idempotency_key, status, created_at, updated_at
FROM events
WHERE idempotency_key = $1;

-- name: GetDeadEvents :many
SELECT id, topic, payload, idempotency_key, status, created_at, updated_at
FROM events
WHERE status = 'dead'
  AND (sqlc.narg('before_id')::text IS NULL OR id < sqlc.narg('before_id'))
ORDER BY updated_at DESC
LIMIT sqlc.arg('limit_count');

-- name: CountEventsByStatus :many
SELECT status, COUNT(*)::bigint AS count
FROM events
GROUP BY status;

-- name: BulkReplayDeadEvents :execrows
UPDATE events SET status = 'pending', updated_at = NOW()
WHERE status = 'dead';

-- name: PurgeDeadEvents :execrows
DELETE FROM events WHERE status = 'dead';
