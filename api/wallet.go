package api

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kys20548/template_golang_web/errcode"
)

// getMyWallet 查詢登入者自己的錢包：user 來源是驗證層放入 context 的
// AuthUser，而不是 request 參數，client 無法指定查別人的錢包。
//
// @Summary  查詢自己的錢包
// @Tags     wallet
// @Produce  json
// @Security TokenAuth
// @Success  200 {object} Response{data=db.Wallet}
// @Router   /wallet [get]
func (server *Server) getMyWallet(ctx *gin.Context) {
	user := getAuthUser(ctx)

	wallet, err := server.store.GetWalletByUserID(ctx, user.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fail(ctx, http.StatusNotFound, errcode.ErrWalletNotFound, nil)
			return
		}
		failInternal(ctx, err)
		return
	}

	ok(ctx, wallet)
}
