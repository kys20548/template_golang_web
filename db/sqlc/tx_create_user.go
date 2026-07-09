package db

import "context"

type CreateUserTxParams struct {
	CreateUserParams
}

type CreateUserTxResult struct {
	User   User   `json:"user"`
	Wallet Wallet `json:"wallet"`
}

// CreateUserTx 在同一個 transaction 中建立使用者與其錢包，
// 任一步失敗都會 rollback，不會出現有 user 沒錢包的狀態。
func (store *SQLStore) CreateUserTx(ctx context.Context, arg CreateUserTxParams) (CreateUserTxResult, error) {
	var result CreateUserTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		result.User, err = q.CreateUser(ctx, arg.CreateUserParams)
		if err != nil {
			return err
		}

		result.Wallet, err = q.CreateWallet(ctx, result.User.ID)
		return err
	})

	return result, err
}
