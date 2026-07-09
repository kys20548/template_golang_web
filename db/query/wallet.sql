-- name: CreateWallet :one
INSERT INTO wallets (
    user_id
) VALUES (
    $1
) RETURNING *;

-- name: GetWalletByUserID :one
SELECT * FROM wallets
WHERE user_id = $1 LIMIT 1;
