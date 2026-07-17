package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/kys20548/template_golang_web/cache"
	db "github.com/kys20548/template_golang_web/db/sqlc"
	"github.com/kys20548/template_golang_web/errcode"
	"github.com/kys20548/template_golang_web/util"
)

const (
	// maxLoginAttempts 次登入失敗後鎖定 loginFailTTL
	maxLoginAttempts = 5
	loginFailTTL     = 15 * time.Minute
	loginFailPrefix  = "login_fail:"
)

func loginFailKey(username string) string {
	return loginFailPrefix + username
}

type loginRequest struct {
	Username string `json:"username" binding:"required,alphanum"`
	Password string `json:"password" binding:"required,min=6"`
}

type loginResponse struct {
	Token string            `json:"token"`
	User  adminUserResponse `json:"user"`
}

// login 驗證後台 user 的帳號密碼後產生 token，把 user 資訊存入 Redis。
// 本專案是後台系統，登入者一律是 admin_users；前台 user（users 表）不在這裡登入。
// 帳號不存在與密碼錯誤回同一個錯誤碼，避免洩漏帳號是否存在。
//
// @Summary  登入
// @Tags     auth
// @Accept   json
// @Produce  json
// @Param    body body loginRequest true "帳號密碼"
// @Success  200 {object} Response{data=loginResponse}
// @Router   /login [post]
func (server *Server) login(ctx *gin.Context) {
	var req loginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	// 失敗次數達上限則鎖定
	failCount, err := server.getLoginFailCount(ctx, req.Username)
	if err != nil {
		failInternal(ctx, err)
		return
	}
	if failCount >= maxLoginAttempts {
		fail(ctx, http.StatusTooManyRequests, errcode.ErrTooManyLoginFails, nil)
		return
	}

	user, err := server.store.GetAdminUserByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			server.recordLoginFail(ctx, req.Username)
			fail(ctx, http.StatusUnauthorized, errcode.ErrWrongCredentials, nil)
			return
		}
		failInternal(ctx, err)
		return
	}

	if err := util.CheckPassword(req.Password, user.HashedPassword); err != nil {
		server.recordLoginFail(ctx, req.Username)
		fail(ctx, http.StatusUnauthorized, errcode.ErrWrongCredentials, nil)
		return
	}

	// 登入成功，清除失敗計數
	if err := server.cache.Del(ctx, loginFailKey(req.Username)); err != nil {
		getLogger(ctx).Warn().Err(err).Msg("cannot clear login fail count")
	}

	// 權限快照：登入時查一次 DB，之後每個 request 從 session 拿、零 DB 查詢；
	// 改角色要重新登入才生效（取捨見 NOTES.md「驗證層」）
	permissions, err := server.store.ListPermissionCodesByAdminUserID(ctx, user.ID)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	token := uuid.NewString()
	authUser := AuthUser{
		UserID:      user.ID,
		Username:    user.Username,
		Permissions: permissions,
	}
	payload, err := json.Marshal(authUser)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	// 一人一 session：反查索引上已有舊 token 就先踢掉（重複登入視同換裝置），
	// 索引才能維持單一 key。踢失敗只記 log，不影響本次登入
	oldToken, err := server.cache.Get(ctx, adminSessionKey(user.ID))
	if err == nil {
		if err := server.cache.Del(ctx, sessionKey(oldToken)); err != nil {
			getLogger(ctx).Warn().Err(err).Msg("cannot delete previous session on re-login")
		}
	} else if !errors.Is(err, cache.ErrNotFound) {
		getLogger(ctx).Warn().Err(err).Msg("cannot check admin session index on login")
	}

	err = server.cache.Set(ctx, sessionKey(token), string(payload), server.config.TokenDuration)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	// 反查索引（user → token）跟 session 同壽命；寫失敗就整個登入失敗，
	// 不能留下「有 session 但反查不到」的帳號（刪除帳號時會踢不掉）
	err = server.cache.Set(ctx, adminSessionKey(user.ID), token, server.config.TokenDuration)
	if err != nil {
		if delErr := server.cache.Del(ctx, sessionKey(token)); delErr != nil {
			getLogger(ctx).Warn().Err(delErr).Msg("cannot rollback session after index write failure")
		}
		failInternal(ctx, err)
		return
	}

	ok(ctx, loginResponse{Token: token, User: newAdminUserResponse(user)})
}

// logout 刪除 Redis 上的 session 與反查索引，token 立即失效。
// 能走到這裡表示已通過 authMiddleware，header 上必有有效 token。
//
// @Summary  登出
// @Tags     auth
// @Produce  json
// @Security TokenAuth
// @Success  200 {object} Response
// @Router   /logout [post]
func (server *Server) logout(ctx *gin.Context) {
	token := ctx.GetHeader(tokenHeaderKey)
	authUser := getAuthUser(ctx)
	if err := server.cache.Del(ctx, sessionKey(token), adminSessionKey(authUser.UserID)); err != nil {
		failInternal(ctx, err)
		return
	}
	ok(ctx, nil)
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=6"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// changePassword 修改登入者（後台 user）自己的密碼，成功後刪除目前的 session
// 與反查索引，需重新登入。
//
// @Summary  修改密碼（成功後目前 token 失效，需重新登入）
// @Tags     auth
// @Accept   json
// @Produce  json
// @Security TokenAuth
// @Param    body body changePasswordRequest true "舊密碼與新密碼"
// @Success  200 {object} Response
// @Failure  400 {object} Response "舊密碼錯誤或參數不合法"
// @Router   /me/password [put]
func (server *Server) changePassword(ctx *gin.Context) {
	var req changePasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	authUser := getAuthUser(ctx)

	user, err := server.store.GetAdminUser(ctx, authUser.UserID)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	if err := util.CheckPassword(req.OldPassword, user.HashedPassword); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrWrongCredentials, nil)
		return
	}

	hashedPassword, err := util.HashPassword(req.NewPassword)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	err = server.store.UpdateAdminUserPassword(ctx, db.UpdateAdminUserPasswordParams{
		ID:             authUser.UserID,
		HashedPassword: hashedPassword,
	})
	if err != nil {
		failInternal(ctx, err)
		return
	}

	// 刪除目前的 session 與反查索引，強制重新登入；失敗只記 log——
	// 密碼已改成功，這個 session 本來就是本人的，留著到過期也無害
	token := ctx.GetHeader(tokenHeaderKey)
	if err := server.cache.Del(ctx, sessionKey(token), adminSessionKey(authUser.UserID)); err != nil {
		getLogger(ctx).Warn().Err(err).Msg("cannot delete session after password change")
	}

	getLogger(ctx).Info().Int64("user_id", authUser.UserID).Msg("password changed")
	ok(ctx, nil)
}

// me 示範從 context 取出驗證層放入的登入者資訊。
// log 用 getLogger 取 request-scoped logger，自動帶 request_id。
//
// @Summary  取得登入者資訊
// @Tags     auth
// @Produce  json
// @Security TokenAuth
// @Success  200 {object} Response{data=AuthUser}
// @Router   /me [get]
func (server *Server) me(ctx *gin.Context) {
	user := getAuthUser(ctx)
	getLogger(ctx).Info().
		Int64("user_id", user.UserID).
		Str("username", user.Username).
		Msg("get current user")

	ok(ctx, user)
}

func (server *Server) getLoginFailCount(ctx *gin.Context, username string) (int64, error) {
	val, err := server.cache.Get(ctx, loginFailKey(username))
	if err != nil {
		if errors.Is(err, cache.ErrNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return strconv.ParseInt(val, 10, 64)
}

func (server *Server) recordLoginFail(ctx *gin.Context, username string) {
	if _, err := server.cache.Incr(ctx, loginFailKey(username), loginFailTTL); err != nil {
		getLogger(ctx).Warn().Err(err).Msg("cannot record login fail count")
	}
}

// kickAdminSession 透過反查索引把指定後台 user 的 session 踢下線，
// 供刪除帳號等「立即失效」場景使用。索引不存在（沒登入）不算錯誤；
// 其餘失敗只記 log，由呼叫端決定主流程要不要繼續。
func (server *Server) kickAdminSession(ctx *gin.Context, adminUserID int64) {
	token, err := server.cache.Get(ctx, adminSessionKey(adminUserID))
	if err != nil {
		if !errors.Is(err, cache.ErrNotFound) {
			getLogger(ctx).Warn().Err(err).Int64("admin_user_id", adminUserID).Msg("cannot look up admin session index")
		}
		return
	}
	if err := server.cache.Del(ctx, sessionKey(token), adminSessionKey(adminUserID)); err != nil {
		getLogger(ctx).Warn().Err(err).Int64("admin_user_id", adminUserID).Msg("cannot kick admin session")
	}
}
