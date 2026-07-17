package api

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	mockcache "github.com/kys20548/template_golang_web/cache/mock"
	mockdb "github.com/kys20548/template_golang_web/db/mock"
	"github.com/kys20548/template_golang_web/errcode"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestDashboardStatsAPI 測 GET /dashboard/stats——統計依登入者權限個別過濾，
// 無對應權限的欄位為 null 且不打對應查詢。
func TestDashboardStatsAPI(t *testing.T) {
	adminUser, _ := testAdminUser(t)

	// 回應欄位用指標：null 與 0 是不同語意（無權限 vs 統計為 0）
	type statsBody struct {
		UserCount           *int64 `json:"user_count"`
		WalletBalanceTotal  *int64 `json:"wallet_balance_total"`
		TodayOperationCount *int64 `json:"today_operation_count"`
	}

	testCases := []struct {
		name          string
		permissions   []string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:        "AllPerms",
			permissions: []string{"*"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					CountUsers(gomock.Any(), gomock.Eq(false)).
					Times(1).
					Return(int64(42), nil)
				store.EXPECT().
					SumWalletBalances(gomock.Any()).
					Times(1).
					Return(int64(12345), nil)
				// 今日起點在 handler 內用當下時間計算，比對不了精確值
				store.EXPECT().
					CountOperationLogsSince(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(7), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var got statsBody
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &got))
				require.NotNil(t, got.UserCount)
				require.Equal(t, int64(42), *got.UserCount)
				require.NotNil(t, got.WalletBalanceTotal)
				require.Equal(t, int64(12345), *got.WalletBalanceTotal)
				require.NotNil(t, got.TodayOperationCount)
				require.Equal(t, int64(7), *got.TodayOperationCount)
			},
		},
		{
			name:        "OnlyUserRead",
			permissions: []string{"user:read"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					CountUsers(gomock.Any(), gomock.Eq(false)).
					Times(1).
					Return(int64(42), nil)
				store.EXPECT().SumWalletBalances(gomock.Any()).Times(0)
				store.EXPECT().CountOperationLogsSince(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var got statsBody
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &got))
				require.NotNil(t, got.UserCount)
				require.Equal(t, int64(42), *got.UserCount)
				require.Nil(t, got.WalletBalanceTotal)
				require.Nil(t, got.TodayOperationCount)
			},
		},
		{
			name:        "NoPerms",
			permissions: []string{},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().CountUsers(gomock.Any(), gomock.Any()).Times(0)
				store.EXPECT().SumWalletBalances(gomock.Any()).Times(0)
				store.EXPECT().CountOperationLogsSince(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var got statsBody
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &got))
				require.Nil(t, got.UserCount)
				require.Nil(t, got.WalletBalanceTotal)
				require.Nil(t, got.TodayOperationCount)
			},
		},
		{
			name:        "InternalError",
			permissions: []string{"*"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					CountUsers(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				require.Equal(t, errcode.ErrInternal, parseResponse(t, recorder.Body, nil))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := mockdb.NewMockStore(ctrl)
			cacheMock := mockcache.NewMockCache(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store, cacheMock)
			recorder := httptest.NewRecorder()

			request, err := http.NewRequest(http.MethodGet, "/dashboard/stats", nil)
			require.NoError(t, err)
			setupAuth(t, request, cacheMock, toAuthUserWithPerms(adminUser, tc.permissions))

			server.Router().ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
