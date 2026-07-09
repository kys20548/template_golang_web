CREATE TABLE "wallets" (
    "id" bigserial PRIMARY KEY,
    "user_id" bigint UNIQUE NOT NULL REFERENCES "users" ("id"),
    "balance" bigint NOT NULL DEFAULT 0,
    "created_at" timestamptz NOT NULL DEFAULT (now())
);

-- 為既有使用者補建錢包
INSERT INTO "wallets" ("user_id")
SELECT "id" FROM "users"
ON CONFLICT ("user_id") DO NOTHING;
