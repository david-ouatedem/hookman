-- name: CreateDeliveryAttempt :exec
INSERT INTO delivery_attempts (id, event_id, endpoint_id, attempt_number, http_status, response_body, duration_ms, status, attempted_at, next_retry_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), $9);

-- name: GetDeliveryAttemptsByEvent :many
SELECT id, event_id, endpoint_id, attempt_number, http_status, response_body, duration_ms, status, attempted_at, next_retry_at
FROM delivery_attempts
WHERE event_id = $1
ORDER BY attempted_at ASC;

-- name: GetDeliveryAttemptsByEndpoint :many
SELECT id, event_id, endpoint_id, attempt_number, http_status, response_body, duration_ms, status, attempted_at, next_retry_at
FROM delivery_attempts
WHERE endpoint_id = $1
ORDER BY attempted_at DESC
LIMIT $2;

-- name: GetRetryableAttempts :many
SELECT DISTINCT ON (da.event_id, da.endpoint_id)
  da.id, da.event_id, da.endpoint_id, da.attempt_number, da.http_status,
  da.response_body, da.duration_ms, da.status, da.attempted_at, da.next_retry_at
FROM delivery_attempts da
JOIN events e ON e.id = da.event_id
WHERE da.next_retry_at IS NOT NULL
  AND da.next_retry_at <= $1
  AND da.status = 'failed'
  AND e.status != 'dead'
ORDER BY da.event_id, da.endpoint_id, da.attempt_number DESC
LIMIT $2;
