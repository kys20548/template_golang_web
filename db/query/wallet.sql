-- name: CreateWallet :one
INSERT INTO wallets (
    user_id
) VALUES (
    $1
) RETURNING *;

-- name: ListWallets :many
SELECT w.id, w.user_id, u.username, u.email, w.balance, w.created_at
FROM wallets w
JOIN users u ON u.id = w.user_id
ORDER BY w.id
LIMIT $1
OFFSET $2;

-- name: CountWallets :one
SELECT count(*) FROM wallets;
