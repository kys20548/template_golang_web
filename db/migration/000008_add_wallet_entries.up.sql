-- 錢包異動帳本：一筆加扣款一列，金額正為加款、負為扣款。
-- 餘額檢查不在這張表——由 wallets 的單句條件 UPDATE 保證併發安全。
CREATE TABLE "wallet_entries" (
    "id" bigserial PRIMARY KEY,
    "wallet_id" bigint NOT NULL REFERENCES "wallets" ("id"),
    "amount" bigint NOT NULL,
    "note" varchar NOT NULL DEFAULT '',
    "operator_id" bigint NOT NULL REFERENCES "admin_users" ("id"),
    "operator_username" varchar NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT (now())
);

-- 明細頁固定以 wallet_id 查詢
CREATE INDEX "wallet_entries_wallet_id_idx" ON "wallet_entries" ("wallet_id");

-- 新權限：錢包加扣款
INSERT INTO "permissions" ("code", "description") VALUES
    ('wallet:write', '錢包加扣款');
