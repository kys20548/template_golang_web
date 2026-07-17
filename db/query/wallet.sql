-- name: CreateWallet :one
INSERT INTO wallets (
    user_id
) VALUES (
    $1
) RETURNING *;

-- 錢包列表只顯示未刪除的前台使用者
-- name: ListWallets :many
SELECT w.id, w.user_id, u.username, u.email, w.balance, w.created_at
FROM wallets w
JOIN users u ON u.id = w.user_id
WHERE u.deleted_at IS NULL
ORDER BY w.id
LIMIT $1
OFFSET $2;

-- name: CountWallets :one
SELECT count(*)
FROM wallets w
JOIN users u ON u.id = w.user_id
WHERE u.deleted_at IS NULL;

-- Dashboard 統計：錢包總餘額（與列表同樣只算未刪除的前台使用者）
-- name: SumWalletBalances :one
SELECT COALESCE(sum(w.balance), 0)::bigint
FROM wallets w
JOIN users u ON u.id = w.user_id
WHERE u.deleted_at IS NULL;

-- name: GetWallet :one
SELECT * FROM wallets
WHERE id = $1;

-- 明細頁抬頭：連同使用者帳號一起回。不過濾軟刪除——使用者刪了帳本仍要可查
-- name: GetWalletDetail :one
SELECT w.id, w.user_id, u.username, u.email, w.balance, w.created_at
FROM wallets w
JOIN users u ON u.id = w.user_id
WHERE w.id = $1;

-- 加扣款與餘額檢查用同一句 UPDATE 保證併發安全：
-- 兩個併發扣款各自原子地檢查「扣完不為負」，不夠扣的那筆條件不成立回 0 rows
-- name: AdjustWalletBalance :one
UPDATE wallets
SET balance = balance + sqlc.arg(amount)
WHERE id = sqlc.arg(id) AND balance + sqlc.arg(amount) >= 0
RETURNING *;
