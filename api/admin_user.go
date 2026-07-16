package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	db "github.com/kys20548/template_golang_web/db/sqlc"
	"github.com/kys20548/template_golang_web/errcode"
)

// adminUserResponse 為後台 user 的對外回應結構，排除 hashed_password 等敏感欄位。
type adminUserResponse struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

func newAdminUserResponse(user db.AdminUser) adminUserResponse {
	return adminUserResponse{
		ID:        user.ID,
		Username:  user.Username,
		CreatedAt: user.CreatedAt,
	}
}

type listAdminUsersRequest struct {
	PageNum  int32 `form:"pageNum" binding:"required,min=1"`
	PageSize int32 `form:"pageSize" binding:"required,min=5,max=50"`
}

// listAdminUsers 分頁查詢後台 user 列表。
//
// @Summary  後台使用者列表
// @Tags     admin-user
// @Produce  json
// @Security TokenAuth
// @Param    pageNum  query int true "頁碼（從 1 開始）"
// @Param    pageSize query int true "每頁筆數（5-50）"
// @Success  200 {object} Response{data=PageResult{list=[]adminUserResponse}}
// @Router   /admin-users [get]
func (server *Server) listAdminUsers(ctx *gin.Context) {
	var req listAdminUsersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	arg := db.ListAdminUsersParams{
		Limit:  req.PageSize,
		Offset: (req.PageNum - 1) * req.PageSize,
	}

	users, err := server.store.ListAdminUsers(ctx, arg)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	total, err := server.store.CountAdminUsers(ctx)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	list := make([]adminUserResponse, 0, len(users))
	for _, user := range users {
		list = append(list, newAdminUserResponse(user))
	}

	ok(ctx, PageResult{
		PageNum:  req.PageNum,
		PageSize: req.PageSize,
		Total:    total,
		List:     list,
	})
}
