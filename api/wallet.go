package api

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	db "github.com/kys20548/template_golang_web/db/sqlc"
	"github.com/kys20548/template_golang_web/errcode"
)

type listWalletsRequest struct {
	PageNum  int32 `form:"pageNum" binding:"required,min=1"`
	PageSize int32 `form:"pageSize" binding:"required,min=5,max=50"`
}

// listWallets 分頁查詢所有前台 user 的錢包（join users 帶出帳號資訊），
// 供後台檢視；wallets 與前台 users 一對一，後台 user 沒有錢包。
//
// @Summary  錢包列表（前台 user）
// @Tags     wallet
// @Produce  json
// @Security TokenAuth
// @Param    pageNum  query int true "頁碼（從 1 開始）"
// @Param    pageSize query int true "每頁筆數（5-50）"
// @Success  200 {object} Response{data=PageResult{list=[]db.ListWalletsRow}}
// @Router   /wallets [get]
func (server *Server) listWallets(ctx *gin.Context) {
	var req listWalletsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	arg := db.ListWalletsParams{
		Limit:  req.PageSize,
		Offset: (req.PageNum - 1) * req.PageSize,
	}

	wallets, err := server.store.ListWallets(ctx, arg)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	total, err := server.store.CountWallets(ctx)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	ok(ctx, PageResult{
		PageNum:  req.PageNum,
		PageSize: req.PageSize,
		Total:    total,
		List:     wallets,
	})
}

type walletUriRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// getWallet 查單一錢包（含使用者帳號資訊），供明細頁抬頭使用。
// 不過濾軟刪除使用者——人刪了帳本仍要可查。
//
// @Summary  錢包單筆查詢
// @Tags     wallet
// @Produce  json
// @Security TokenAuth
// @Param    id path int true "錢包 ID"
// @Success  200 {object} Response{data=db.GetWalletDetailRow}
// @Failure  404 {object} Response "錢包不存在"
// @Router   /wallets/{id} [get]
func (server *Server) getWallet(ctx *gin.Context) {
	var uri walletUriRequest
	if err := ctx.ShouldBindUri(&uri); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	wallet, err := server.store.GetWalletDetail(ctx, uri.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fail(ctx, http.StatusNotFound, errcode.ErrNotFound, nil)
			return
		}
		failInternal(ctx, err)
		return
	}

	ok(ctx, wallet)
}

type adjustWalletRequest struct {
	// Amount 正為加款、負為扣款；required 同時擋掉 0
	Amount int64  `json:"amount" binding:"required"`
	Note   string `json:"note" binding:"max=255"`
}

// adjustWallet 對錢包加扣款：調整餘額並寫入 wallet_entries 帳本（同一個 transaction）。
// 餘額檢查由單句條件 UPDATE 保證併發安全，不夠扣回 30001 餘額不足。
//
// @Summary  錢包加扣款
// @Tags     wallet
// @Accept   json
// @Produce  json
// @Security TokenAuth
// @Param    id   path int true "錢包 ID"
// @Param    body body adjustWalletRequest true "金額（正加負扣、不可為 0）與備註"
// @Success  200 {object} Response{data=db.AdjustWalletTxResult}
// @Failure  400 {object} Response "餘額不足（code 30001）"
// @Failure  404 {object} Response "錢包不存在"
// @Router   /wallets/{id}/adjust [post]
func (server *Server) adjustWallet(ctx *gin.Context) {
	var uri walletUriRequest
	if err := ctx.ShouldBindUri(&uri); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	var req adjustWalletRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	operator := getAuthUser(ctx)
	result, err := server.store.AdjustWalletTx(ctx, db.AdjustWalletTxParams{
		WalletID:         uri.ID,
		Amount:           req.Amount,
		Note:             req.Note,
		OperatorID:       operator.UserID,
		OperatorUsername: operator.Username,
	})
	if err != nil {
		if errors.Is(err, db.ErrInsufficientBalance) {
			fail(ctx, http.StatusBadRequest, errcode.ErrInsufficientBalance, nil)
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			fail(ctx, http.StatusNotFound, errcode.ErrNotFound, nil)
			return
		}
		failInternal(ctx, err)
		return
	}

	getLogger(ctx).Info().
		Int64("wallet_id", uri.ID).
		Int64("amount", req.Amount).
		Int64("balance", result.Wallet.Balance).
		Msg("wallet adjusted")
	ok(ctx, result)
}

type listWalletEntriesRequest struct {
	PageNum  int32 `form:"pageNum" binding:"required,min=1"`
	PageSize int32 `form:"pageSize" binding:"required,min=5,max=50"`
}

// listWalletEntries 分頁查詢單一錢包的異動明細，最新的在前。
//
// @Summary  錢包異動明細列表
// @Tags     wallet
// @Produce  json
// @Security TokenAuth
// @Param    id       path  int true "錢包 ID"
// @Param    pageNum  query int true "頁碼（從 1 開始）"
// @Param    pageSize query int true "每頁筆數（5-50）"
// @Success  200 {object} Response{data=PageResult{list=[]db.WalletEntry}}
// @Router   /wallets/{id}/entries [get]
func (server *Server) listWalletEntries(ctx *gin.Context) {
	var uri walletUriRequest
	if err := ctx.ShouldBindUri(&uri); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	var req listWalletEntriesRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	entries, err := server.store.ListWalletEntries(ctx, db.ListWalletEntriesParams{
		WalletID: uri.ID,
		Limit:    req.PageSize,
		Offset:   (req.PageNum - 1) * req.PageSize,
	})
	if err != nil {
		failInternal(ctx, err)
		return
	}

	total, err := server.store.CountWalletEntries(ctx, uri.ID)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	ok(ctx, PageResult{
		PageNum:  req.PageNum,
		PageSize: req.PageSize,
		Total:    total,
		List:     entries,
	})
}
