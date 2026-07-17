-- name: CreateUser :one
INSERT INTO users (
    username,
    email,
    hashed_password
) VALUES (
    $1, $2, $3
) RETURNING *;

-- GetUser 不過濾 deleted_at：後台以 ID 查詳情時，已刪除者也要能查到
-- name: GetUser :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = $1 AND deleted_at IS NULL
LIMIT 1;

-- name: ListUsers :many
SELECT * FROM users
WHERE (sqlc.arg(include_deleted)::bool OR deleted_at IS NULL)
ORDER BY id
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: CountUsers :one
SELECT count(*) FROM users
WHERE (sqlc.arg(include_deleted)::bool OR deleted_at IS NULL);

-- name: SoftDeleteUser :execrows
UPDATE users
SET deleted_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreUser :execrows
UPDATE users
SET deleted_at = NULL
WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: UpdateUserPassword :exec
UPDATE users
SET hashed_password = $2
WHERE id = $1;
