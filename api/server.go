package api

import (
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kys20548/template_golang_web/cache"
	db "github.com/kys20548/template_golang_web/db/sqlc"
	"github.com/kys20548/template_golang_web/util"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Server 負責處理所有 HTTP 請求。
type Server struct {
	config util.Config
	store  db.Store
	cache  cache.Cache
	router *gin.Engine
}

// NewServer 建立 HTTP server 並設定路由。
func NewServer(config util.Config, store db.Store, cacheStore cache.Cache) (*Server, error) {
	server := &Server{
		config: config,
		store:  store,
		cache:  cacheStore,
	}

	server.setupRouter()
	return server, nil
}

func (server *Server) setupRouter() {
	if server.config.Environment != "development" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 不用 gin.Default()：以 zerolog middleware 取代 gin 內建 logger。
	// recovery 用 CustomRecoveryWithWriter：panic 一律回統一格式給 client；
	// server 端的 stack 在 development 用 gin 內建可讀格式印出，
	// production 則丟掉 plain text，由 recoveryHandler 把 stack 記進 zerolog JSON
	var recoveryWriter io.Writer = gin.DefaultErrorWriter
	if server.config.Environment != "development" {
		recoveryWriter = io.Discard
	}

	router := gin.New()

	// gin 預設 ctx.Done()/Deadline() 回空值，開 fallback 才會委派給
	// Request.Context()，timeoutMiddleware 的 deadline 才能傳進
	// 直接收 *gin.Context 的 sqlc / go-redis 呼叫
	router.ContextWithFallback = true

	router.Use(
		requestIDMiddleware(),
		httpLogger(),
		timeoutMiddleware(server.config.APITimeout),
		corsMiddleware(server.config),
		gin.CustomRecoveryWithWriter(recoveryWriter, recoveryHandler),
		auditLogMiddleware(server.store),
	)

	// Swagger 文件只在 development 提供，production 不對外暴露 API 結構
	if server.config.Environment == "development" {
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// 公開路由
	router.GET("/healthz", server.healthCheck)
	// readiness 探針要快進快出：掛更短的 timeout，DB 連不上時 2s 內就回 503，
	// 不佔著探針等全域的 10s
	router.GET("/readyz", timeoutMiddleware(2*time.Second), server.readyCheck)
	router.POST("/users", server.createUser)
	router.POST("/login", server.login)

	// 需要驗證的路由：header 帶 token，經 authMiddleware 驗證後才會進到 handler；
	// 資源類路由再各自掛 permMiddleware 檢查權限（登入時的快照，見 NOTES.md「驗證層」）
	authRoutes := router.Group("/").Use(authMiddleware(server.cache, server.config.TokenDuration))
	// 只要登入就能用：自己的 session 與密碼
	authRoutes.POST("/logout", server.logout)
	authRoutes.GET("/me", server.me)
	authRoutes.PUT("/me/password", server.changePassword)
	// 統計卡片不掛 permMiddleware：回應內各統計依登入者權限個別過濾
	authRoutes.GET("/dashboard/stats", server.dashboardStats)
	// 資源查詢／管理：依 resource:action 檢查權限
	authRoutes.GET("/wallets", permMiddleware("wallet:read"), server.listWallets)
	authRoutes.GET("/wallets/:id", permMiddleware("wallet:read"), server.getWallet)
	authRoutes.GET("/wallets/:id/entries", permMiddleware("wallet:read"), server.listWalletEntries)
	authRoutes.POST("/wallets/:id/adjust", permMiddleware("wallet:write"), server.adjustWallet)
	authRoutes.GET("/users/:id", permMiddleware("user:read"), server.getUser)
	// 個別路由範例：超過 2s 印 slow request WARN log（不中斷請求）
	authRoutes.GET("/users", permMiddleware("user:read"), slowLogMiddleware(2000*time.Millisecond), server.listUsers)
	authRoutes.DELETE("/users/:id", permMiddleware("user:write"), server.deleteUser)
	authRoutes.PUT("/users/:id/restore", permMiddleware("user:write"), server.restoreUser)
	authRoutes.GET("/admin-users", permMiddleware("admin_user:read"), server.listAdminUsers)
	authRoutes.POST("/admin-users", permMiddleware("admin_user:write"), server.createAdminUser)
	authRoutes.PUT("/admin-users/:id/roles", permMiddleware("admin_user:write"), server.updateAdminUserRoles)
	authRoutes.DELETE("/admin-users/:id", permMiddleware("admin_user:write"), server.deleteAdminUser)
	authRoutes.PUT("/admin-users/:id/restore", permMiddleware("admin_user:write"), server.restoreAdminUser)
	authRoutes.GET("/roles", permMiddleware("admin_user:read"), server.listRoles)
	authRoutes.GET("/operation-logs", permMiddleware("operation_log:read"), server.listOperationLogs)

	server.router = router
}

// Router 回傳 gin engine，讓 main 可以把它掛到 http.Server 上做 graceful shutdown。
func (server *Server) Router() *gin.Engine {
	return server.router
}
