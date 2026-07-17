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

// adminUserResponse 為後台 user 的對外回應結構，排除 hashed_password 等敏感欄位。
// DeletedAt 非 null 表示已被軟刪除（列表帶 includeDeleted=true 才查得到）。
type adminUserResponse struct {
	ID        int64      `json:"id"`
	Username  string     `json:"username"`
	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at"`
}

func newAdminUserResponse(user db.AdminUser) adminUserResponse {
	resp := adminUserResponse{
		ID:        user.ID,
		Username:  user.Username,
		CreatedAt: user.CreatedAt,
	}
	if user.DeletedAt.Valid {
		resp.DeletedAt = &user.DeletedAt.Time
	}
	return resp
}

// roleBrief 為掛在後台 user 身上的角色摘要。
type roleBrief struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// adminUserWithRolesResponse 為後台 user 列表的單筆回應：帳號 + 角色摘要。
type adminUserWithRolesResponse struct {
	adminUserResponse
	Roles []roleBrief `json:"roles"`
}

type listAdminUsersRequest struct {
	PageNum        int32 `form:"pageNum" binding:"required,min=1"`
	PageSize       int32 `form:"pageSize" binding:"required,min=5,max=50"`
	IncludeDeleted bool  `form:"includeDeleted"`
}

// listAdminUsers 分頁查詢後台 user 列表，每筆帶角色摘要。
// 預設只列未刪除者，includeDeleted=true 連已刪除的一起列。
// 角色關聯是一次撈全部再在記憶體組裝——後台帳號數量小，不值得為分頁做陣列參數查詢。
//
// @Summary  後台使用者列表（含角色）
// @Tags     admin-user
// @Produce  json
// @Security TokenAuth
// @Param    pageNum  query int true "頁碼（從 1 開始）"
// @Param    pageSize query int true "每頁筆數（5-50）"
// @Param    includeDeleted query bool false "是否包含已刪除者"
// @Success  200 {object} Response{data=PageResult{list=[]adminUserWithRolesResponse}}
// @Router   /admin-users [get]
func (server *Server) listAdminUsers(ctx *gin.Context) {
	var req listAdminUsersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	arg := db.ListAdminUsersParams{
		IncludeDeleted: req.IncludeDeleted,
		PageLimit:      req.PageSize,
		PageOffset:     (req.PageNum - 1) * req.PageSize,
	}

	users, err := server.store.ListAdminUsers(ctx, arg)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	total, err := server.store.CountAdminUsers(ctx, req.IncludeDeleted)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	userRoles, err := server.store.ListAdminUserRoles(ctx)
	if err != nil {
		failInternal(ctx, err)
		return
	}
	rolesByUser := make(map[int64][]roleBrief)
	for _, ur := range userRoles {
		rolesByUser[ur.AdminUserID] = append(rolesByUser[ur.AdminUserID], roleBrief{ID: ur.RoleID, Name: ur.Name})
	}

	list := make([]adminUserWithRolesResponse, 0, len(users))
	for _, user := range users {
		roles := rolesByUser[user.ID]
		if roles == nil {
			roles = []roleBrief{}
		}
		list = append(list, adminUserWithRolesResponse{
			adminUserResponse: newAdminUserResponse(user),
			Roles:             roles,
		})
	}

	ok(ctx, PageResult{
		PageNum:  req.PageNum,
		PageSize: req.PageSize,
		Total:    total,
		List:     list,
	})
}

type createAdminUserRequest struct {
	Username string  `json:"username" binding:"required,alphanum"`
	Password string  `json:"password" binding:"required,min=6"`
	RoleIDs  []int64 `json:"role_ids" binding:"omitempty,dive,min=1"`
}

// createAdminUser 建立後台 user 並指派角色（同一個 transaction）。
//
// @Summary  建立後台使用者
// @Tags     admin-user
// @Accept   json
// @Produce  json
// @Security TokenAuth
// @Param    body body createAdminUserRequest true "帳號、密碼與角色"
// @Success  200 {object} Response{data=adminUserResponse}
// @Router   /admin-users [post]
func (server *Server) createAdminUser(ctx *gin.Context) {
	var req createAdminUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	hashedPassword, err := util.HashPassword(req.Password)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	result, err := server.store.CreateAdminUserTx(ctx, db.CreateAdminUserTxParams{
		CreateAdminUserParams: db.CreateAdminUserParams{
			Username:       req.Username,
			HashedPassword: hashedPassword,
		},
		RoleIDs: req.RoleIDs,
	})
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			switch pqErr.Code.Name() {
			case "unique_violation":
				fail(ctx, http.StatusConflict, errcode.ErrUserExists, nil)
				return
			case "foreign_key_violation": // role_ids 指到不存在的角色
				fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
				return
			}
		}
		failInternal(ctx, err)
		return
	}

	getLogger(ctx).Info().Int64("admin_user_id", result.AdminUser.ID).Msg("admin user created")
	ok(ctx, newAdminUserResponse(result.AdminUser))
}

type updateAdminUserRolesUriRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

type updateAdminUserRolesRequest struct {
	RoleIDs []int64 `json:"role_ids" binding:"omitempty,dive,min=1"`
}

// updateAdminUserRoles 以「整組取代」更新後台 user 的角色。
// 對方已登入的 session 不受影響（權限是登入時的快照），重新登入後生效。
//
// @Summary  指派後台使用者角色（整組取代）
// @Tags     admin-user
// @Accept   json
// @Produce  json
// @Security TokenAuth
// @Param    id   path int true "後台使用者 ID"
// @Param    body body updateAdminUserRolesRequest true "角色 ID 清單（空陣列 = 清空角色）"
// @Success  200 {object} Response
// @Router   /admin-users/{id}/roles [put]
func (server *Server) updateAdminUserRoles(ctx *gin.Context) {
	var uri updateAdminUserRolesUriRequest
	if err := ctx.ShouldBindUri(&uri); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	var req updateAdminUserRolesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	// 先確認帳號存在，才不會對不存在的 user 靜默成功
	if _, err := server.store.GetAdminUser(ctx, uri.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fail(ctx, http.StatusNotFound, errcode.ErrUserNotFound, nil)
			return
		}
		failInternal(ctx, err)
		return
	}

	err := server.store.UpdateAdminUserRolesTx(ctx, db.UpdateAdminUserRolesTxParams{
		AdminUserID: uri.ID,
		RoleIDs:     req.RoleIDs,
	})
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code.Name() == "foreign_key_violation" {
			fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
			return
		}
		failInternal(ctx, err)
		return
	}

	getLogger(ctx).Info().Int64("admin_user_id", uri.ID).Ints64("role_ids", req.RoleIDs).Msg("admin user roles updated")
	ok(ctx, nil)
}

type deleteAdminUserRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// deleteAdminUser 軟刪除後台使用者，並透過反查索引把對方的 session 立即踢下線。
// 不能刪除自己——這同時保證系統裡永遠至少留著一個能還原帳號的操作者。
//
// @Summary  刪除後台使用者（軟刪除，session 立即失效）
// @Tags     admin-user
// @Produce  json
// @Security TokenAuth
// @Param    id path int true "後台使用者 ID"
// @Success  200 {object} Response
// @Failure  400 {object} Response "不能刪除自己"
// @Failure  404 {object} Response "使用者不存在或已刪除"
// @Router   /admin-users/{id} [delete]
func (server *Server) deleteAdminUser(ctx *gin.Context) {
	var req deleteAdminUserRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	if req.ID == getAuthUser(ctx).UserID {
		fail(ctx, http.StatusBadRequest, errcode.ErrCannotDeleteSelf, nil)
		return
	}

	rows, err := server.store.SoftDeleteAdminUser(ctx, req.ID)
	if err != nil {
		failInternal(ctx, err)
		return
	}
	if rows == 0 {
		fail(ctx, http.StatusNotFound, errcode.ErrUserNotFound, nil)
		return
	}

	// 帳號已標記刪除，把對方現有 session 踢下線（沒登入就沒事）
	server.kickAdminSession(ctx, req.ID)

	getLogger(ctx).Info().Int64("admin_user_id", req.ID).Msg("admin user soft deleted")
	ok(ctx, nil)
}

// restoreAdminUser 還原已軟刪除的後台使用者，角色綁定原封不動地回來。
// 若刪除期間有人建立了同名帳號，還原會撞 partial unique index 回 409。
//
// @Summary  還原後台使用者
// @Tags     admin-user
// @Produce  json
// @Security TokenAuth
// @Param    id path int true "後台使用者 ID"
// @Success  200 {object} Response
// @Failure  404 {object} Response "使用者不存在或未被刪除"
// @Failure  409 {object} Response "帳號名已被重新建立"
// @Router   /admin-users/{id}/restore [put]
func (server *Server) restoreAdminUser(ctx *gin.Context) {
	var req deleteAdminUserRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	rows, err := server.store.RestoreAdminUser(ctx, req.ID)
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

	getLogger(ctx).Info().Int64("admin_user_id", req.ID).Msg("admin user restored")
	ok(ctx, nil)
}
