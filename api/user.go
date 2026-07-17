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
// DeletedAt 非 null 表示已被軟刪除（列表帶 includeDeleted=true 才查得到）。
type userResponse struct {
	ID        int64      `json:"id"`
	Username  string     `json:"username"`
	Email     string     `json:"email"`
	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at"`
}

func newUserResponse(user db.User) userResponse {
	resp := userResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}
	if user.DeletedAt.Valid {
		resp.DeletedAt = &user.DeletedAt.Time
	}
	return resp
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
	PageNum        int32 `form:"pageNum" binding:"required,min=1"`
	PageSize       int32 `form:"pageSize" binding:"required,min=5,max=50"`
	IncludeDeleted bool  `form:"includeDeleted"`
}

// listUsers 分頁查詢使用者列表，預設只列未刪除者，includeDeleted=true 連已刪除的一起列。
//
// @Summary  使用者列表
// @Tags     user
// @Produce  json
// @Security TokenAuth
// @Param    pageNum  query int true "頁碼（從 1 開始）"
// @Param    pageSize query int true "每頁筆數（5-50）"
// @Param    includeDeleted query bool false "是否包含已刪除者"
// @Success  200 {object} Response{data=PageResult{list=[]userResponse}}
// @Router   /users [get]
func (server *Server) listUsers(ctx *gin.Context) {
	var req listUsersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	arg := db.ListUsersParams{
		IncludeDeleted: req.IncludeDeleted,
		PageLimit:      req.PageSize,
		PageOffset:     (req.PageNum - 1) * req.PageSize,
	}

	users, err := server.store.ListUsers(ctx, arg)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	total, err := server.store.CountUsers(ctx, req.IncludeDeleted)
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

type deleteUserRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// deleteUser 軟刪除前台使用者（deleted_at 打上時間戳）：
// 錢包等關聯資料保留，可透過還原 API 復原。
//
// @Summary  刪除前台使用者（軟刪除）
// @Tags     user
// @Produce  json
// @Security TokenAuth
// @Param    id path int true "使用者 ID"
// @Success  200 {object} Response
// @Failure  404 {object} Response "使用者不存在或已刪除"
// @Router   /users/{id} [delete]
func (server *Server) deleteUser(ctx *gin.Context) {
	var req deleteUserRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	rows, err := server.store.SoftDeleteUser(ctx, req.ID)
	if err != nil {
		failInternal(ctx, err)
		return
	}
	if rows == 0 {
		fail(ctx, http.StatusNotFound, errcode.ErrUserNotFound, nil)
		return
	}

	getLogger(ctx).Info().Int64("user_id", req.ID).Msg("user soft deleted")
	ok(ctx, nil)
}

// restoreUser 還原已軟刪除的前台使用者。
// 若刪除期間有人註冊了同名帳號或同 email，還原會撞 partial unique index 回 409。
//
// @Summary  還原前台使用者
// @Tags     user
// @Produce  json
// @Security TokenAuth
// @Param    id path int true "使用者 ID"
// @Success  200 {object} Response
// @Failure  404 {object} Response "使用者不存在或未被刪除"
// @Failure  409 {object} Response "帳號名或 email 已被重新註冊"
// @Router   /users/{id}/restore [put]
func (server *Server) restoreUser(ctx *gin.Context) {
	var req deleteUserRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	rows, err := server.store.RestoreUser(ctx, req.ID)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code.Name() == "unique_violation" {
			fail(ctx, http.StatusConflict, errcode.ErrUserExists, err)
			return
		}
		failInternal(ctx, err)
		return
	}
	if rows == 0 {
		fail(ctx, http.StatusNotFound, errcode.ErrUserNotFound, nil)
		return
	}

	getLogger(ctx).Info().Int64("user_id", req.ID).Msg("user restored")
	ok(ctx, nil)
}
