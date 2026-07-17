package db

import (
	"context"
	"database/sql"
	"fmt"
)

// Store 提供所有 DB 操作（單一 query 與 transaction）。
// 內嵌 sqlc 產生的 Querier interface，方便 mock 測試。
type Store interface {
	Querier
	CreateUserTx(ctx context.Context, arg CreateUserTxParams) (CreateUserTxResult, error)
	CreateAdminUserTx(ctx context.Context, arg CreateAdminUserTxParams) (CreateAdminUserTxResult, error)
	UpdateAdminUserRolesTx(ctx context.Context, arg UpdateAdminUserRolesTxParams) error
	// Ping 檢查 DB 連線是否正常，供啟動檢查與 readiness 探針使用。
	Ping(ctx context.Context) error
}

// SQLStore 為 Store 的實際實作，操作真實的 PostgreSQL。
type SQLStore struct {
	*Queries
	db *sql.DB
}

func NewStore(db *sql.DB) Store {
	return &SQLStore{
		db:      db,
		Queries: New(db),
	}
}

func (store *SQLStore) Ping(ctx context.Context) error {
	return store.db.PingContext(ctx)
}

// execTx 在一個 database transaction 中執行 fn。
func (store *SQLStore) execTx(ctx context.Context, fn func(*Queries) error) error {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	q := New(tx)
	err = fn(q)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx err: %v, rb err: %v", err, rbErr)
		}
		return err
	}

	return tx.Commit()
}
