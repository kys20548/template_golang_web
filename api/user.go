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

func (server *Server) healthCheck(ctx *gin.Context) {
	ok(ctx, "ok")
}

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

func (server *Server) createUser(ctx *gin.Context) {
	var req createUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	hashedPassword, err := util.HashPassword(req.Password)
	if err != nil {
		fail(ctx, http.StatusInternalServerError, errcode.ErrInternal, err)
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
		fail(ctx, http.StatusInternalServerError, errcode.ErrInternal, err)
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
		fail(ctx, http.StatusInternalServerError, errcode.ErrInternal, err)
		return
	}

	ok(ctx, newUserResponse(user))
}

type listUsersRequest struct {
	PageNum  int32 `form:"pageNum" binding:"required,min=1"`
	PageSize int32 `form:"pageSize" binding:"required,min=5,max=50"`
}

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
		fail(ctx, http.StatusInternalServerError, errcode.ErrInternal, err)
		return
	}

	total, err := server.store.CountUsers(ctx)
	if err != nil {
		fail(ctx, http.StatusInternalServerError, errcode.ErrInternal, err)
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
