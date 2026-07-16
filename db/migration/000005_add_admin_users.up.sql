CREATE TABLE "admin_users" (
    "id" bigserial PRIMARY KEY,
    "username" varchar UNIQUE NOT NULL,
    "hashed_password" varchar NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT (now())
);

-- 種子後台帳號 admin / admin123，部署後請立即修改密碼
INSERT INTO "admin_users" ("username", "hashed_password")
VALUES ('admin', '$2a$10$4.tLK5H8VtM7AGTrab4SSeYNdrM7M9kEUVvanw9zUMB9SKIC7Kiuu');
