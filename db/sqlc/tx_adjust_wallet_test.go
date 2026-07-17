package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// testStore 連本地開發 DB（make postgres && make migrateup 之後存在）。
// 連不上就 skip——mock 驗不了「單句條件 UPDATE 的併發安全」，這條必須打真 DB。
func testStore(t *testing.T) Store {
	source := os.Getenv("DB_SOURCE")
	if source == "" {
		source = "postgresql://root:secret@localhost:5432/template_golang_web?sslmode=disable"
	}

	conn, err := sql.Open("postgres", source)
	require.NoError(t, err)
	if err := conn.Ping(); err != nil {
		t.Skipf("本地 DB 未啟動，跳過併發測試: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return NewStore(conn)
}

// TestAdjustWalletTxConcurrent 驗證併發扣款的正確性：
// 餘額 500，10 個 goroutine 同時各扣 100——必須恰好 5 筆成功、5 筆餘額不足，
// 最終餘額 0、帳本筆數與成功筆數一致，不能出現負餘額或多扣/少扣。
func TestAdjustWalletTxConcurrent(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	// 專屬測試資料：唯一帳號避免撞既有資料；operator 用種子帳號 admin（FK 需要）
	operator, err := store.GetAdminUserByUsername(ctx, "admin")
	require.NoError(t, err)

	suffix := time.Now().UnixNano()
	created, err := store.CreateUserTx(ctx, CreateUserTxParams{
		CreateUserParams: CreateUserParams{
			Username:       fmt.Sprintf("wallet_tx_%d", suffix),
			Email:          fmt.Sprintf("wallet_tx_%d@example.com", suffix),
			HashedPassword: "not-a-real-hash",
		},
	})
	require.NoError(t, err)
	walletID := created.Wallet.ID

	// 先加款 500 當本金
	_, err = store.AdjustWalletTx(ctx, AdjustWalletTxParams{
		WalletID:         walletID,
		Amount:           500,
		Note:             "併發測試本金",
		OperatorID:       operator.ID,
		OperatorUsername: operator.Username,
	})
	require.NoError(t, err)

	// 10 個 goroutine 同時各扣 100
	const n = 10
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.AdjustWalletTx(ctx, AdjustWalletTxParams{
				WalletID:         walletID,
				Amount:           -100,
				Note:             "併發扣款",
				OperatorID:       operator.ID,
				OperatorUsername: operator.Username,
			})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	succeeded, insufficient := 0, 0
	for err := range errs {
		switch {
		case err == nil:
			succeeded++
		case err == ErrInsufficientBalance:
			insufficient++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	require.Equal(t, 5, succeeded)
	require.Equal(t, 5, insufficient)

	// 最終餘額歸零、帳本筆數 = 1 筆本金 + 5 筆成功扣款
	wallet, err := store.GetWallet(ctx, walletID)
	require.NoError(t, err)
	require.Equal(t, int64(0), wallet.Balance)

	total, err := store.CountWalletEntries(ctx, walletID)
	require.NoError(t, err)
	require.Equal(t, int64(6), total)
}
