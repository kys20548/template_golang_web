package api

import (
	"io"

	"github.com/gin-gonic/gin"
	"github.com/kys20548/template_golang_web/cache"
	db "github.com/kys20548/template_golang_web/db/sqlc"
	"github.com/kys20548/template_golang_web/util"
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
	router.Use(
		requestIDMiddleware(),
		httpLogger(),
		corsMiddleware(server.config),
		gin.CustomRecoveryWithWriter(recoveryWriter, recoveryHandler),
	)

	// 公開路由
	router.GET("/healthz", server.healthCheck)
	router.POST("/users", server.createUser)
	router.POST("/login", server.login)

	// 需要驗證的路由：header 帶 token，經 authMiddleware 驗證後才會進到 handler
	authRoutes := router.Group("/").Use(authMiddleware(server.cache))
	authRoutes.POST("/logout", server.logout)
	authRoutes.GET("/me", server.me)
	authRoutes.GET("/wallet", server.getMyWallet)
	authRoutes.GET("/users/:id", server.getUser)
	authRoutes.GET("/users", server.listUsers)

	server.router = router
}

// Router 回傳 gin engine，讓 main 可以把它掛到 http.Server 上做 graceful shutdown。
func (server *Server) Router() *gin.Engine {
	return server.router
}
