package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kys20548/template_golang_web/errcode"
)

// Response 為所有 API 的統一回應格式。
type Response struct {
	Code errcode.Code `json:"code"`
	Msg  string       `json:"msg"`
	Data any          `json:"data"`
}

// PageResult 為分頁列表 API 的統一回應主體，放在 Response.Data 裡。
type PageResult struct {
	PageNum  int32 `json:"pageNum"`
	PageSize int32 `json:"pageSize"`
	Total    int64 `json:"total"`
	List     any   `json:"list"`
}

// ok 回傳成功回應，data 為該 API 的回應主體。
func ok(ctx *gin.Context, data any) {
	ctx.JSON(http.StatusOK, Response{
		Code: errcode.Success,
		Msg:  errcode.Success.Msg(),
		Data: data,
	})
}

// fail 回傳錯誤回應。err 用於 log 記錄（可為 nil），不會回傳給 client，
// client 只會看到 errcode 定義的統一訊息。
func fail(ctx *gin.Context, httpStatus int, code errcode.Code, err error) {
	if err != nil {
		ctx.Error(err)
	}
	ctx.JSON(httpStatus, Response{
		Code: code,
		Msg:  code.Msg(),
		Data: nil,
	})
}

// failAbort 同 fail，但中斷後續 middleware / handler，供驗證層使用。
func failAbort(ctx *gin.Context, httpStatus int, code errcode.Code, err error) {
	if err != nil {
		ctx.Error(err)
	}
	ctx.AbortWithStatusJSON(httpStatus, Response{
		Code: code,
		Msg:  code.Msg(),
		Data: nil,
	})
}
