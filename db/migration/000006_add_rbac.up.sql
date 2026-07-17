CREATE TABLE "roles" (
    "id" bigserial PRIMARY KEY,
    "name" varchar UNIQUE NOT NULL,
    "description" varchar NOT NULL DEFAULT '',
    "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "permissions" (
    "id" bigserial PRIMARY KEY,
    "code" varchar UNIQUE NOT NULL,
    "description" varchar NOT NULL DEFAULT '',
    "created_at" timestamptz NOT NULL DEFAULT (now())
);

CREATE TABLE "role_permissions" (
    "role_id" bigint NOT NULL REFERENCES "roles" ("id") ON DELETE CASCADE,
    "permission_id" bigint NOT NULL REFERENCES "permissions" ("id") ON DELETE CASCADE,
    PRIMARY KEY ("role_id", "permission_id")
);

CREATE TABLE "admin_user_roles" (
    "admin_user_id" bigint NOT NULL REFERENCES "admin_users" ("id") ON DELETE CASCADE,
    "role_id" bigint NOT NULL REFERENCES "roles" ("id") ON DELETE CASCADE,
    PRIMARY KEY ("admin_user_id", "role_id")
);

-- 權限清單：code 用 resource:action，* 為萬用（super_admin 專用）
INSERT INTO "permissions" ("code", "description") VALUES
    ('*',                  '全部權限'),
    ('user:read',          '查詢前台使用者'),
    ('admin_user:read',    '查詢後台使用者與角色'),
    ('admin_user:write',   '新增後台使用者、指派角色'),
    ('wallet:read',        '查詢錢包'),
    ('operation_log:read', '查詢操作日誌');

INSERT INTO "roles" ("name", "description") VALUES
    ('super_admin', '超級管理員（全部權限）'),
    ('viewer',      '唯讀（前台使用者/錢包/操作日誌）');

INSERT INTO "role_permissions" ("role_id", "permission_id")
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'super_admin' AND p.code = '*';

INSERT INTO "role_permissions" ("role_id", "permission_id")
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'viewer' AND p.code IN ('user:read', 'wallet:read', 'operation_log:read');

-- 種子帳號 admin 指派 super_admin
INSERT INTO "admin_user_roles" ("admin_user_id", "role_id")
SELECT au.id, r.id FROM admin_users au, roles r
WHERE au.username = 'admin' AND r.name = 'super_admin';
