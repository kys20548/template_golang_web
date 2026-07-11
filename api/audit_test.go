package api

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mockcache "github.com/kys20548/template_golang_web/cache/mock"
	mockdb "github.com/kys20548/template_golang_web/db/mock"
	db "github.com/kys20548/template_golang_web/db/sqlc"
	"github.com/kys20548/template_golang_web/errcode"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestListOperationLogsAPI(t *testing.T) {
	user, _ := testUser(t)
	logs := []db.OperationLog{
		{
			ID:        2,
			UserID:    sql.NullInt64{Int64: user.ID, Valid: true},
			Username:  user.Username,
			Method:    http.MethodPost,
			Path:      "/logout",
			CreatedAt: time.Now(),
		},
		{
			ID:        1,
			Method:    http.MethodPost, // 未登入的操作：UserID 是 NULL
			Path:      "/login",
			CreatedAt: time.Now(),
		},
	}

	ctrl := gomock.NewController(t)
	store := mockdb.NewMockStore(ctrl)
	cacheMock := mockcache.NewMockCache(ctrl)

	store.EXPECT().
		ListOperationLogs(gomock.Any(), gomock.Eq(db.ListOperationLogsParams{Limit: 10, Offset: 0})).
		Times(1).
		Return(logs, nil)
	store.EXPECT().
		CountOperationLogs(gomock.Any()).
		Times(1).
		Return(int64(2), nil)

	server := newTestServer(t, store, cacheMock)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/operation-logs?pageNum=1&pageSize=10", nil)
	require.NoError(t, err)
	setupAuth(t, request, cacheMock, toAuthUser(user))

	server.Router().ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var got PageResult
	require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &got))
	require.Equal(t, int64(2), got.Total)

	// 對外結構 operationLogResponse 的 user_id 用指標表達「未登入為 null」
	body := recorder.Body.String()
	require.Contains(t, body, `"user_id":1`)
	require.Contains(t, body, `"user_id":null`)
}
