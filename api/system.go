package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kys20548/template_golang_web/errcode"
)

// healthCheck liveness 檢查：進程活著就回 200，不看依賴。
//
// @Summary  健康檢查（liveness）
// @Tags     system
// @Produce  json
// @Success  200 {object} Response{data=string}
// @Router   /healthz [get]
func (server *Server) healthCheck(ctx *gin.Context) {
	ok(ctx, "ok")
}

// readyCheck readiness 檢查：ping DB 與 Redis，供 LB / ASG(ELB health check) /
// k8s readiness probe 判斷這台能不能收流量。依賴掛了回 503，讓 LB 摘掉流量
// 等依賴恢復——不是讓進程自己退出（自殺重啟會把一次 DB 抖動放大成全軍覆沒）。
//
// @Summary  Readiness 檢查（含 DB/Redis 連線）
// @Tags     system
// @Produce  json
// @Success  200 {object} Response{data=string}
// @Failure  503 {object} Response
// @Router   /readyz [get]
func (server *Server) readyCheck(ctx *gin.Context) {
	if err := server.store.Ping(ctx); err != nil {
		fail(ctx, http.StatusServiceUnavailable, errcode.ErrNotReady, fmt.Errorf("db not ready: %w", err))
		return
	}
	if err := server.cache.Ping(ctx); err != nil {
		fail(ctx, http.StatusServiceUnavailable, errcode.ErrNotReady, fmt.Errorf("redis not ready: %w", err))
		return
	}
	ok(ctx, "ready")
}
