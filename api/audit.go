package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	db "github.com/kys20548/template_golang_web/db/sqlc"
	"github.com/kys20548/template_golang_web/errcode"
)

// maxAuditBodySize 為寫進 operation_logs 的 request body 上限，超過的截斷。
const maxAuditBodySize = 4096

// auditLogMiddleware 操作日誌：記錄所有寫入類操作（POST/PUT/PATCH/DELETE）
// 的誰（登入者）、什麼時候、改了什麼（method + path + body）與結果 status code。
// 寫入失敗只記 log，不影響已回應的請求。
func auditLogMiddleware(store db.Store) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if !isWriteMethod(ctx.Request.Method) {
			ctx.Next()
			return
		}

		requestBody := readAndRestoreBody(ctx)

		ctx.Next()

		// 走到這裡表示 handler 已處理完；驗證通過的請求 context 會有登入者資訊
		var userID sql.NullInt64
		var username string
		if v, exists := ctx.Get(authUserKey); exists {
			user := v.(AuthUser)
			userID = sql.NullInt64{Int64: user.UserID, Valid: true}
			username = user.Username
		}

		arg := db.CreateOperationLogParams{
			UserID:      userID,
			Username:    username,
			Method:      ctx.Request.Method,
			Path:        ctx.Request.URL.Path,
			RequestBody: maskSensitiveFields(requestBody),
			StatusCode:  int32(ctx.Writer.Status()),
			RequestID:   ctx.GetString(requestIDKey),
		}

		// 用 WithoutCancel：請求本身超時被取消時，稽核紀錄仍要寫得進去
		c := context.WithoutCancel(ctx.Request.Context())
		if _, err := store.CreateOperationLog(c, arg); err != nil {
			getLogger(ctx).Error().Err(err).Msg("cannot write operation log")
		}
	}
}

func isWriteMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

// readAndRestoreBody 讀出 request body 供稽核用，並放一份回去讓 handler 照常讀取。
// 回傳值最多保留 maxAuditBodySize bytes。
func readAndRestoreBody(ctx *gin.Context) string {
	if ctx.Request.Body == nil {
		return ""
	}

	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		return ""
	}
	ctx.Request.Body = io.NopCloser(bytes.NewReader(body))

	if len(body) > maxAuditBodySize {
		body = body[:maxAuditBodySize]
	}
	return string(body)
}

// maskSensitiveFields 遮罩 JSON body 第一層 key 含 password 的欄位，
// 避免明文密碼被寫進操作日誌。非 JSON 的 body 原樣回傳。
func maskSensitiveFields(body string) string {
	if body == "" {
		return ""
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return body
	}

	masked := false
	for key := range payload {
		if strings.Contains(strings.ToLower(key), "password") {
			payload[key] = "***"
			masked = true
		}
	}
	if !masked {
		return body
	}

	result, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return string(result)
}

// operationLogResponse 為操作日誌的對外回應結構，user_id 用指標表達「未登入為 null」。
type operationLogResponse struct {
	ID          int64  `json:"id"`
	UserID      *int64 `json:"user_id"`
	Username    string `json:"username"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	RequestBody string `json:"request_body"`
	StatusCode  int32  `json:"status_code"`
	RequestID   string `json:"request_id"`
	CreatedAt   string `json:"created_at"`
}

func newOperationLogResponse(operationLog db.OperationLog) operationLogResponse {
	resp := operationLogResponse{
		ID:          operationLog.ID,
		Username:    operationLog.Username,
		Method:      operationLog.Method,
		Path:        operationLog.Path,
		RequestBody: operationLog.RequestBody,
		StatusCode:  operationLog.StatusCode,
		RequestID:   operationLog.RequestID,
		CreatedAt:   operationLog.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if operationLog.UserID.Valid {
		resp.UserID = &operationLog.UserID.Int64
	}
	return resp
}

type listOperationLogsRequest struct {
	PageNum  int32 `form:"pageNum" binding:"required,min=1"`
	PageSize int32 `form:"pageSize" binding:"required,min=5,max=50"`
}

// listOperationLogs 分頁查詢操作日誌，最新的在前。
//
// @Summary  操作日誌列表
// @Tags     system
// @Produce  json
// @Security TokenAuth
// @Param    pageNum  query int true "頁碼（從 1 開始）"
// @Param    pageSize query int true "每頁筆數（5-50）"
// @Success  200 {object} Response{data=PageResult{list=[]operationLogResponse}}
// @Router   /operation-logs [get]
func (server *Server) listOperationLogs(ctx *gin.Context) {
	var req listOperationLogsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	arg := db.ListOperationLogsParams{
		Limit:  req.PageSize,
		Offset: (req.PageNum - 1) * req.PageSize,
	}

	logs, err := server.store.ListOperationLogs(ctx, arg)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	total, err := server.store.CountOperationLogs(ctx)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	list := make([]operationLogResponse, 0, len(logs))
	for _, operationLog := range logs {
		list = append(list, newOperationLogResponse(operationLog))
	}

	ok(ctx, PageResult{
		PageNum:  req.PageNum,
		PageSize: req.PageSize,
		Total:    total,
		List:     list,
	})
}
