-- name: ListAdminUserRoles :many
SELECT aur.admin_user_id, r.id AS role_id, r.name
FROM admin_user_roles aur
JOIN roles r ON r.id = aur.role_id
ORDER BY aur.admin_user_id, r.id;

-- name: CreateAdminUserRole :exec
INSERT INTO admin_user_roles (
    admin_user_id,
    role_id
) VALUES (
    $1, $2
);

-- name: DeleteAdminUserRoles :exec
DELETE FROM admin_user_roles
WHERE admin_user_id = $1;
