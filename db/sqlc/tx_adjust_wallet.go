package db

import (
	"context"
	"database/sql"
	"errors"
)

// ErrInsufficientBalance 表示扣款金額超過餘額。
// AdjustWalletTx 用它區分「餘額不足」與「錢包不存在」（後者回 sql.ErrNoRows）。
var ErrInsufficientBalance = errors.New("insufficient wallet balance")

type AdjustWalletTxParams struct {
	WalletID int64
	// Amount 正為加款、負為扣款
	Amount           int64
	Note             string
	OperatorID       int64
	OperatorUsername string
}

type AdjustWalletTxResult struct {
	Wallet Wallet      `json:"wallet"`
	Entry  WalletEntry `json:"entry"`
}

// AdjustWalletTx 在同一個 transaction 中調整餘額並寫入帳本，任一步失敗都 rollback。
// 併發安全靠 AdjustWalletBalance 單句 UPDATE 的條件（balance + amount >= 0）：
// 條件不成立回 0 rows，此時查一次錢包區分是餘額不足還是錢包不存在。
func (store *SQLStore) AdjustWalletTx(ctx context.Context, arg AdjustWalletTxParams) (AdjustWalletTxResult, error) {
	var result AdjustWalletTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		result.Wallet, err = q.AdjustWalletBalance(ctx, AdjustWalletBalanceParams{
			Amount: arg.Amount,
			ID:     arg.WalletID,
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				if _, getErr := q.GetWallet(ctx, arg.WalletID); getErr == nil {
					return ErrInsufficientBalance
				}
			}
			return err
		}

		result.Entry, err = q.CreateWalletEntry(ctx, CreateWalletEntryParams{
			WalletID:         arg.WalletID,
			Amount:           arg.Amount,
			Note:             arg.Note,
			OperatorID:       arg.OperatorID,
			OperatorUsername: arg.OperatorUsername,
		})
		return err
	})

	return result, err
}
