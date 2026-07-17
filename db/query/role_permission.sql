-- name: ListRolePermissions :many
SELECT rp.role_id, p.code, p.description
FROM role_permissions rp
JOIN permissions p ON p.id = rp.permission_id
ORDER BY rp.role_id, p.code;
