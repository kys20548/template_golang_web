package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/kys20548/template_golang_web/cache"
	"github.com/kys20548/template_golang_web/errcode"
	"github.com/kys20548/template_golang_web/util"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	// tokenHeaderKey 為 client 帶 token 的 header 名稱。
	tokenHeaderKey = "token"
	// authUserKey 為驗證通過後 user 資訊存放在 gin context 的 key。
	authUserKey = "auth_user"
	// sessionKeyPrefix 為 token 存在 Redis 的 key 前綴。
	sessionKeyPrefix = "session:"
	// requestIDHeaderKey 為 request id 的 header 名稱，回應時帶回給 client。
	requestIDHeaderKey = "X-Request-Id"
	// requestIDKey 為 request id 存放在 gin context 的 key，也是 log 欄位名。
	requestIDKey = "request_id"
)

// AuthUser 為登入者（後台 user）資訊，驗證通過後放入 context 供後續邏輯使用。
// Permissions 是登入當下的權限快照（permission codes）——之後改角色要重新登入
// 才生效，取捨同 session 不做反查索引（見 NOTES.md「驗證層」）。
type AuthUser struct {
	UserID      int64    `json:"user_id"`
	Username    string   `json:"username"`
	Permissions []string `json:"permissions"`
}

// HasPermission 檢查是否具備某權限；permWildcard 代表全部權限。
func (u AuthUser) HasPermission(code string) bool {
	for _, p := range u.Permissions {
		if p == permWildcard || p == code {
			return true
		}
	}
	return false
}

func sessionKey(token string) string {
	return sessionKeyPrefix + token
}

// authMiddleware 驗證層：從 header 取出 token，確認存在於 Redis 後
// 將 user 資訊放入 context，否則中斷請求。
// 驗證通過時做 sliding TTL：把 session 的存活時間重設回 tokenDuration——
// 活躍使用者不會用到一半被登出，閒置滿 tokenDuration 才過期。
func authMiddleware(cacheStore cache.Cache, tokenDuration time.Duration) gin.HandlerFunc {
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

		// sliding TTL：續期失敗不影響本次請求（session 還沒過期），只記 log
		if err := cacheStore.Expire(ctx, sessionKey(token), tokenDuration); err != nil {
			getLogger(ctx).Warn().Err(err).Msg("cannot refresh session ttl")
		}

		ctx.Set(authUserKey, user)
		ctx.Next()
	}
}

// getAuthUser 從 context 取出驗證層放入的登入者資訊。
func getAuthUser(ctx *gin.Context) AuthUser {
	return ctx.MustGet(authUserKey).(AuthUser)
}

// permWildcard 為萬用權限 code，super_admin 角色持有。
const permWildcard = "*"

// permMiddleware 權限層：掛在 authMiddleware 之後的個別路由上，
// 檢查登入者的權限快照是否含指定 code（或萬用 *），否則 403。
func permMiddleware(code string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if !getAuthUser(ctx).HasPermission(code) {
			failAbort(ctx, http.StatusForbidden, errcode.ErrForbidden, nil)
			return
		}
		ctx.Next()
	}
}

// requestIDMiddleware 為每個請求產生唯一的 request id（client 有帶就沿用，
// 方便跨服務串接），放進 response header 回給 client，並把帶有 request_id
// 的 logger 放進 request context，同一請求的所有 log 都能用它串起來。
// 對應 Java 的 MDC + traceId。
func requestIDMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := ctx.GetHeader(requestIDHeaderKey)
		if requestID == "" {
			requestID = uuid.NewString()
		}

		ctx.Set(requestIDKey, requestID)
		ctx.Header(requestIDHeaderKey, requestID)

		logger := log.With().Str(requestIDKey, requestID).Logger()
		ctx.Request = ctx.Request.WithContext(logger.WithContext(ctx.Request.Context()))

		ctx.Next()
	}
}

// getLogger 取得帶 request_id 的 request-scoped logger，
// handler 內記 log 一律用它，不要直接用全域的 log。
func getLogger(ctx *gin.Context) *zerolog.Logger {
	return log.Ctx(ctx.Request.Context())
}

// timeoutMiddleware 把 deadline 掛在 request context 上，超時會取消
// 進行中的 DB / Redis 操作，handler 收到 context.DeadlineExceeded 後
// 經 failInternal 回 504。全域掛 API_TIMEOUT；個別路由可再掛更短的，
// 巢狀 context 誰短誰先到期。
// 注意：需搭配 router.ContextWithFallback = true 才能讓 deadline
// 傳進直接收 *gin.Context 的 sqlc / go-redis 呼叫。
func timeoutMiddleware(d time.Duration) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		c, cancel := context.WithTimeout(ctx.Request.Context(), d)
		defer cancel()

		ctx.Request = ctx.Request.WithContext(c)
		ctx.Next()
	}
}

// slowLogMiddleware 個別路由的慢請求門檻：超過 threshold 只印 WARN log
// 方便排查，不中斷請求（硬超時由全域 timeoutMiddleware 負責）。
func slowLogMiddleware(threshold time.Duration) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		start := time.Now()
		ctx.Next()

		if duration := time.Since(start); duration > threshold {
			getLogger(ctx).Warn().
				Str("method", ctx.Request.Method).
				Str("path", ctx.Request.URL.Path).
				Dur("duration", duration).
				Dur("threshold", threshold).
				Msg("slow request")
		}
	}
}

// recoveryHandler 統一 panic 回應：gin.Recovery 只會回空 body 的 500，
// 這裡改成回統一的 {code, msg, data} 格式。
// stack 只在 production 記進 zerolog（單行 JSON 給 log 收集器）；
// development 的 stack 由 gin 內建 writer 以可讀的多行格式印出（見 setupRouter）。
func recoveryHandler(ctx *gin.Context, err any) {
	event := getLogger(ctx).Error().Interface("panic", err)
	if !gin.IsDebugging() {
		event = event.Str("stack", string(debug.Stack()))
	}
	event.Msg("panic recovered")

	failAbort(ctx, http.StatusInternalServerError, errcode.ErrInternal, fmt.Errorf("panic: %v", err))
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
			Str(requestIDKey, ctx.GetString(requestIDKey)).
			Str("method", ctx.Request.Method).
			Str("path", ctx.Request.URL.Path).
			Int("status_code", statusCode).
			Str("status_text", http.StatusText(statusCode)).
			Dur("duration", duration).
			Msg("received a HTTP request")
	}
}
