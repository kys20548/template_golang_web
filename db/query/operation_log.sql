-- name: CreateOperationLog :one
INSERT INTO operation_logs (
    user_id, username, method, path, request_body, status_code, request_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: ListOperationLogs :many
SELECT * FROM operation_logs
ORDER BY id DESC
LIMIT $1
OFFSET $2;

-- name: CountOperationLogs :one
SELECT count(*) FROM operation_logs;
