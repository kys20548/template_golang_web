DELETE FROM "permissions" WHERE "code" = 'user:write';

DROP INDEX "admin_users_username_key";
ALTER TABLE "admin_users" ADD CONSTRAINT "admin_users_username_key" UNIQUE ("username");

DROP INDEX "users_email_key";
DROP INDEX "users_username_key";
ALTER TABLE "users" ADD CONSTRAINT "users_email_key" UNIQUE ("email");
ALTER TABLE "users" ADD CONSTRAINT "users_username_key" UNIQUE ("username");

ALTER TABLE "admin_users" DROP COLUMN "deleted_at";
ALTER TABLE "users" DROP COLUMN "deleted_at";
