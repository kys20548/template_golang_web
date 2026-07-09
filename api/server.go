package api

import (
	"github.com/gin-gonic/gin"
	db "github.com/kys20548/template_golang_web/db/sqlc"
	"github.com/kys20548/template_golang_web/util"
)

// Server 負責處理所有 HTTP 請求。
type Server struct {
	config util.Config
	store  db.Store
	router *gin.Engine
}

// NewServer 建立 HTTP server 並設定路由。
func NewServer(config util.Config, store db.Store) (*Server, error) {
	server := &Server{
		config: config,
		store:  store,
	}

	server.setupRouter()
	return server, nil
}

func (server *Server) setupRouter() {
	if server.config.Environment != "development" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 不用 gin.Default()：以 zerolog middleware 取代 gin 內建 logger
	router := gin.New()
	router.Use(httpLogger(), gin.Recovery())

	router.GET("/healthz", server.healthCheck)

	router.POST("/users", server.createUser)
	router.GET("/users/:id", server.getUser)
	router.GET("/users", server.listUsers)

	server.router = router
}

// Router 回傳 gin engine，讓 main 可以把它掛到 http.Server 上做 graceful shutdown。
func (server *Server) Router() *gin.Engine {
	return server.router
}

func errorResponse(err error) gin.H {
	return gin.H{"error": err.Error()}
}
