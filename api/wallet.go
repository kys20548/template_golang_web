package api

import (
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
