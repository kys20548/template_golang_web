-- name: CreateAdminUser :one
INSERT INTO admin_users (
    username,
    hashed_password
) VALUES (
    $1, $2
) RETURNING *;

-- name: GetAdminUser :one
SELECT * FROM admin_users
WHERE id = $1 AND deleted_at IS NULL
LIMIT 1;

-- name: GetAdminUserByUsername :one
SELECT * FROM admin_users
WHERE username = $1 AND deleted_at IS NULL
LIMIT 1;

-- name: ListAdminUsers :many
SELECT * FROM admin_users
WHERE (sqlc.arg(include_deleted)::bool OR deleted_at IS NULL)
ORDER BY id
LIMIT sqlc.arg(page_limit)
OFFSET sqlc.arg(page_offset);

-- name: CountAdminUsers :one
SELECT count(*) FROM admin_users
WHERE (sqlc.arg(include_deleted)::bool OR deleted_at IS NULL);

-- name: SoftDeleteAdminUser :execrows
UPDATE admin_users
SET deleted_at = now()
WHERE id = $1 AND deleted_at IS NULL;

-- name: RestoreAdminUser :execrows
UPDATE admin_users
SET deleted_at = NULL
WHERE id = $1 AND deleted_at IS NOT NULL;

-- name: UpdateAdminUserPassword :exec
UPDATE admin_users
SET hashed_password = $2
WHERE id = $1;
