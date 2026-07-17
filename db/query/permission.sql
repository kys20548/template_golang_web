-- name: ListPermissionCodesByAdminUserID :many
SELECT DISTINCT p.code
FROM admin_user_roles aur
JOIN role_permissions rp ON rp.role_id = aur.role_id
JOIN permissions p ON p.id = rp.permission_id
WHERE aur.admin_user_id = $1
ORDER BY p.code;
