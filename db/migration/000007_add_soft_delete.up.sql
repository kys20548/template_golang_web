-- 前後台使用者軟刪除：deleted_at 有值即視為已刪除，查詢一律過濾
ALTER TABLE "users" ADD COLUMN "deleted_at" timestamptz;
ALTER TABLE "admin_users" ADD COLUMN "deleted_at" timestamptz;

-- 唯一鍵改成 partial unique index（只約束未刪除者）：
-- 軟刪除後同名帳號可以重新註冊，否則刪掉的帳號會永遠占用 username/email
ALTER TABLE "users" DROP CONSTRAINT "users_username_key";
ALTER TABLE "users" DROP CONSTRAINT "users_email_key";
CREATE UNIQUE INDEX "users_username_key" ON "users" ("username") WHERE "deleted_at" IS NULL;
CREATE UNIQUE INDEX "users_email_key" ON "users" ("email") WHERE "deleted_at" IS NULL;

ALTER TABLE "admin_users" DROP CONSTRAINT "admin_users_username_key";
CREATE UNIQUE INDEX "admin_users_username_key" ON "admin_users" ("username") WHERE "deleted_at" IS NULL;

-- 新權限：刪除／還原前台使用者（後台使用者的刪除沿用 admin_user:write）
INSERT INTO "permissions" ("code", "description") VALUES
    ('user:write', '刪除/還原前台使用者');
