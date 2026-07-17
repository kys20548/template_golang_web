package db

import "context"

type CreateAdminUserTxParams struct {
	CreateAdminUserParams
	RoleIDs []int64
}

type CreateAdminUserTxResult struct {
	AdminUser AdminUser `json:"admin_user"`
}

// CreateAdminUserTx 在同一個 transaction 中建立後台 user 並指派角色，
// 任一步失敗都會 rollback，不會出現有帳號沒角色的狀態。
func (store *SQLStore) CreateAdminUserTx(ctx context.Context, arg CreateAdminUserTxParams) (CreateAdminUserTxResult, error) {
	var result CreateAdminUserTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		result.AdminUser, err = q.CreateAdminUser(ctx, arg.CreateAdminUserParams)
		if err != nil {
			return err
		}

		for _, roleID := range arg.RoleIDs {
			err = q.CreateAdminUserRole(ctx, CreateAdminUserRoleParams{
				AdminUserID: result.AdminUser.ID,
				RoleID:      roleID,
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return result, err
}

type UpdateAdminUserRolesTxParams struct {
	AdminUserID int64
	RoleIDs     []int64
}

// UpdateAdminUserRolesTx 以「整組取代」更新後台 user 的角色：
// 先刪光再逐筆插入，同一個 transaction 保證不會停在半套狀態。
func (store *SQLStore) UpdateAdminUserRolesTx(ctx context.Context, arg UpdateAdminUserRolesTxParams) error {
	return store.execTx(ctx, func(q *Queries) error {
		if err := q.DeleteAdminUserRoles(ctx, arg.AdminUserID); err != nil {
			return err
		}

		for _, roleID := range arg.RoleIDs {
			err := q.CreateAdminUserRole(ctx, CreateAdminUserRoleParams{
				AdminUserID: arg.AdminUserID,
				RoleID:      roleID,
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}
