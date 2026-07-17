-- name: CreateAdminUser :one
INSERT INTO admin_users (
    username,
    hashed_password
) VALUES (
    $1, $2
) RETURNING *;

-- name: GetAdminUser :one
SELECT * FROM admin_users
WHERE id = $1 LIMIT 1;

-- name: GetAdminUserByUsername :one
SELECT * FROM admin_users
WHERE username = $1 LIMIT 1;

-- name: ListAdminUsers :many
SELECT * FROM admin_users
ORDER BY id
LIMIT $1
OFFSET $2;

-- name: CountAdminUsers :one
SELECT count(*) FROM admin_users;

-- name: UpdateAdminUserPassword :exec
UPDATE admin_users
SET hashed_password = $2
WHERE id = $1;
