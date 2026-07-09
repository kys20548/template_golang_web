package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kys20548/template_golang_web/api"
	"github.com/kys20548/template_golang_web/cache"
	db "github.com/kys20548/template_golang_web/db/sqlc"
	_ "github.com/kys20548/template_golang_web/docs"
	"github.com/kys20548/template_golang_web/util"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// @title           template_golang_web API
// @version         1.0
// @description     Golang Web 專案模板 API 文件。所有回應皆為統一格式 {code, msg, data}。
// @host            localhost:8080
// @BasePath        /
//
// @securityDefinitions.apikey TokenAuth
// @in   header
// @name token
// @description 登入後取得的 token，需驗證的 API 都要在 header 帶上
func main() {
	config, err := util.LoadConfig(".")
	if err != nil {
		log.Fatal().Err(err).Msg("cannot load config")
	}

	// development 環境輸出人類可讀格式，production 輸出 JSON
	if config.Environment == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	conn, err := sql.Open(config.DBDriver, config.DBSource)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot open db")
	}
	defer conn.Close()

	// 連線池設定：sql.DB 預設開啟連線數無上限，尖峰時會把 DB 塞爆；
	// ConnMaxLifetime 讓連線定期換新，避免被 DB 或中間的 LB 靜默斷線
	conn.SetMaxOpenConns(config.DBMaxOpenConns)
	conn.SetMaxIdleConns(config.DBMaxIdleConns)
	conn.SetConnMaxLifetime(config.DBConnMaxLifetime)

	// 啟動檢查：DB 或 Redis 連不上就不啟動 HTTP server，
	// 避免起了一個一定會噴錯的服務（sql.Open 是惰性的，要 Ping 才會真正連線）
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.PingContext(pingCtx); err != nil {
		log.Fatal().Err(err).Msg("cannot connect to db")
	}
	log.Info().Msg("db connected")

	store := db.NewStore(conn)

	cacheStore := cache.NewRedisCache(config.RedisAddress)
	if err := cacheStore.Ping(pingCtx); err != nil {
		log.Fatal().Err(err).Msg("cannot connect to redis")
	}
	log.Info().Msg("redis connected")

	server, err := api.NewServer(config, store, cacheStore)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create server")
	}

	httpServer := &http.Server{
		Addr:    config.HTTPServerAddress,
		Handler: server.Router(),
	}

	// 在 goroutine 中啟動 server，main goroutine 負責監聽關閉訊號
	go func() {
		log.Info().Msgf("start HTTP server at %s", config.HTTPServerAddress)
		err := httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("cannot start server")
		}
	}()

	listenSignal(httpServer, config)
}

// listenSignal 阻塞等待 SIGINT / SIGTERM，收到訊號後優雅關閉 server：
// 停止接收新連線，並在 timeout 內等待進行中的請求處理完成。
func listenSignal(server *http.Server, config util.Config) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch // 阻塞，直到收到訊號

	log.Info().Msg("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("server forced to shutdown")
	}

	log.Info().Msg("http server 已安全終止")
}
