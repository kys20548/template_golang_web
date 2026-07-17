package api

import (
	"time"

	"github.com/gin-gonic/gin"
)

// dashboardStatsResponse 為首頁統計卡片的回應。
// 各欄位依登入者權限個別過濾：無對應權限的欄位為 null，前端據此決定顯示哪些卡片。
type dashboardStatsResponse struct {
	// UserCount 前台使用者數（未刪除），需 user:read
	UserCount *int64 `json:"user_count"`
	// WalletBalanceTotal 錢包總餘額（未刪除使用者），需 wallet:read
	WalletBalanceTotal *int64 `json:"wallet_balance_total"`
	// TodayOperationCount 今日操作數（本地時區當天 0 點起），需 operation_log:read
	TodayOperationCount *int64 `json:"today_operation_count"`
}

// dashboardStats 回傳首頁統計卡片數字。
// 不掛 permMiddleware：登入即可打，回應內各統計依權限個別過濾，
// 沒有任何資源權限的登入者拿到三個 null。
//
// @Summary  首頁統計卡片
// @Tags     system
// @Produce  json
// @Security TokenAuth
// @Success  200 {object} Response{data=dashboardStatsResponse}
// @Router   /dashboard/stats [get]
func (server *Server) dashboardStats(ctx *gin.Context) {
	user := getAuthUser(ctx)
	var resp dashboardStatsResponse

	if user.HasPermission("user:read") {
		count, err := server.store.CountUsers(ctx, false)
		if err != nil {
			failInternal(ctx, err)
			return
		}
		resp.UserCount = &count
	}

	if user.HasPermission("wallet:read") {
		total, err := server.store.SumWalletBalances(ctx)
		if err != nil {
			failInternal(ctx, err)
			return
		}
		resp.WalletBalanceTotal = &total
	}

	if user.HasPermission("operation_log:read") {
		now := time.Now()
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		count, err := server.store.CountOperationLogsSince(ctx, startOfDay)
		if err != nil {
			failInternal(ctx, err)
			return
		}
		resp.TodayOperationCount = &count
	}

	ok(ctx, resp)
}
