-- name: CreateWalletEntry :one
INSERT INTO wallet_entries (
    wallet_id, amount, note, operator_id, operator_username
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: ListWalletEntries :many
SELECT * FROM wallet_entries
WHERE wallet_id = $1
ORDER BY id DESC
LIMIT $2
OFFSET $3;

-- name: CountWalletEntries :one
SELECT count(*) FROM wallet_entries
WHERE wallet_id = $1;
