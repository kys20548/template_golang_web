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
	Token string       `json:"token"`
	User  userResponse `json:"user"`
}

// login 驗證帳號密碼後產生 token，把 user 資訊存入 Redis。
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

	user, err := server.store.GetUserByUsername(ctx, req.Username)
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

	token := uuid.NewString()
	authUser := AuthUser{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
	}
	payload, err := json.Marshal(authUser)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	err = server.cache.Set(ctx, sessionKey(token), string(payload), server.config.TokenDuration)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	ok(ctx, loginResponse{Token: token, User: newUserResponse(user)})
}

// logout 刪除 Redis 上的 session，token 立即失效。
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
	if err := server.cache.Del(ctx, sessionKey(token)); err != nil {
		failInternal(ctx, err)
		return
	}
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
