package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/kys20548/template_golang_web/cache"
	"github.com/kys20548/template_golang_web/errcode"
	"github.com/kys20548/template_golang_web/util"
	"github.com/rs/zerolog/log"
)

const (
	// tokenHeaderKey 為 client 帶 token 的 header 名稱。
	tokenHeaderKey = "token"
	// authUserKey 為驗證通過後 user 資訊存放在 gin context 的 key。
	authUserKey = "auth_user"
	// sessionKeyPrefix 為 token 存在 Redis 的 key 前綴。
	sessionKeyPrefix = "session:"
)

// AuthUser 為登入者資訊，驗證通過後放入 context 供後續邏輯使用。
type AuthUser struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

func sessionKey(token string) string {
	return sessionKeyPrefix + token
}

// authMiddleware 驗證層：從 header 取出 token，確認存在於 Redis 後
// 將 user 資訊放入 context，否則中斷請求。
func authMiddleware(cacheStore cache.Cache) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token := ctx.GetHeader(tokenHeaderKey)
		if token == "" {
			failAbort(ctx, http.StatusUnauthorized, errcode.ErrUnauthorized, nil)
			return
		}

		val, err := cacheStore.Get(ctx, sessionKey(token))
		if err != nil {
			if err == cache.ErrNotFound {
				failAbort(ctx, http.StatusUnauthorized, errcode.ErrUnauthorized, nil)
				return
			}
			failAbort(ctx, http.StatusInternalServerError, errcode.ErrInternal, err)
			return
		}

		var user AuthUser
		if err := json.Unmarshal([]byte(val), &user); err != nil {
			failAbort(ctx, http.StatusInternalServerError, errcode.ErrInternal, err)
			return
		}

		ctx.Set(authUserKey, user)
		ctx.Next()
	}
}

// getAuthUser 從 context 取出驗證層放入的登入者資訊。
func getAuthUser(ctx *gin.Context) AuthUser {
	return ctx.MustGet(authUserKey).(AuthUser)
}

// corsMiddleware 依設定允許跨域請求；CORS_ALLOW_ORIGINS 為 * 時允許所有來源，
// 否則為逗號分隔的來源清單。
func corsMiddleware(config util.Config) gin.HandlerFunc {
	corsCfg := cors.DefaultConfig()
	if config.CORSAllowOrigins == "*" {
		corsCfg.AllowAllOrigins = true
	} else {
		corsCfg.AllowOrigins = strings.Split(config.CORSAllowOrigins, ",")
	}
	corsCfg.AllowHeaders = append(corsCfg.AllowHeaders, tokenHeaderKey)
	return cors.New(corsCfg)
}

// httpLogger 以 zerolog 記錄每一筆 HTTP 請求。
func httpLogger() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		startTime := time.Now()
		ctx.Next()
		duration := time.Since(startTime)

		statusCode := ctx.Writer.Status()
		logger := log.Info()
		if statusCode >= http.StatusInternalServerError {
			logger = log.Error()
		}
		if len(ctx.Errors) > 0 {
			logger = logger.Strs("errors", ctx.Errors.Errors())
		}

		logger.Str("protocol", "http").
			Str("method", ctx.Request.Method).
			Str("path", ctx.Request.URL.Path).
			Int("status_code", statusCode).
			Str("status_text", http.StatusText(statusCode)).
			Dur("duration", duration).
			Msg("received a HTTP request")
	}
}
