package api

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	db "github.com/kys20548/template_golang_web/db/sqlc"
	"github.com/kys20548/template_golang_web/errcode"
	"github.com/kys20548/template_golang_web/util"
	"github.com/lib/pq"
)

// userResponse 為 user 的對外回應結構，排除 hashed_password 等敏感欄位。
type userResponse struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func newUserResponse(user db.User) userResponse {
	return userResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}
}

type createUserRequest struct {
	Username string `json:"username" binding:"required,alphanum"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type createUserResponse struct {
	User   userResponse `json:"user"`
	Wallet db.Wallet    `json:"wallet"`
}

// createUser 建立使用者，並在同一個 transaction 內建立錢包。
//
// @Summary  建立使用者
// @Tags     user
// @Accept   json
// @Produce  json
// @Param    body body createUserRequest true "使用者資料"
// @Success  200 {object} Response{data=createUserResponse}
// @Router   /users [post]
func (server *Server) createUser(ctx *gin.Context) {
	var req createUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	hashedPassword, err := util.HashPassword(req.Password)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	arg := db.CreateUserTxParams{
		CreateUserParams: db.CreateUserParams{
			Username:       req.Username,
			Email:          req.Email,
			HashedPassword: hashedPassword,
		},
	}

	// 使用 transaction 同時建立使用者與錢包，任一步失敗都會 rollback
	result, err := server.store.CreateUserTx(ctx, arg)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code.Name() == "unique_violation" {
			fail(ctx, http.StatusConflict, errcode.ErrUserExists, nil)
			return
		}
		failInternal(ctx, err)
		return
	}

	ok(ctx, createUserResponse{
		User:   newUserResponse(result.User),
		Wallet: result.Wallet,
	})
}

type getUserRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// getUser 以 ID 查詢使用者。
//
// @Summary  查詢使用者
// @Tags     user
// @Produce  json
// @Security TokenAuth
// @Param    id path int true "使用者 ID"
// @Success  200 {object} Response{data=userResponse}
// @Router   /users/{id} [get]
func (server *Server) getUser(ctx *gin.Context) {
	var req getUserRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	user, err := server.store.GetUser(ctx, req.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fail(ctx, http.StatusNotFound, errcode.ErrUserNotFound, nil)
			return
		}
		failInternal(ctx, err)
		return
	}

	ok(ctx, newUserResponse(user))
}

type listUsersRequest struct {
	PageNum  int32 `form:"pageNum" binding:"required,min=1"`
	PageSize int32 `form:"pageSize" binding:"required,min=5,max=50"`
}

// listUsers 分頁查詢使用者列表。
//
// @Summary  使用者列表
// @Tags     user
// @Produce  json
// @Security TokenAuth
// @Param    pageNum  query int true "頁碼（從 1 開始）"
// @Param    pageSize query int true "每頁筆數（5-50）"
// @Success  200 {object} Response{data=PageResult{list=[]userResponse}}
// @Router   /users [get]
func (server *Server) listUsers(ctx *gin.Context) {
	var req listUsersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	arg := db.ListUsersParams{
		Limit:  req.PageSize,
		Offset: (req.PageNum - 1) * req.PageSize,
	}

	users, err := server.store.ListUsers(ctx, arg)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	total, err := server.store.CountUsers(ctx)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	list := make([]userResponse, 0, len(users))
	for _, user := range users {
		list = append(list, newUserResponse(user))
	}

	ok(ctx, PageResult{
		PageNum:  req.PageNum,
		PageSize: req.PageSize,
		Total:    total,
		List:     list,
	})
}
