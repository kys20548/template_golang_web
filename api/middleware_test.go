package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kys20548/template_golang_web/cache"
	mockcache "github.com/kys20548/template_golang_web/cache/mock"
	"github.com/kys20548/template_golang_web/errcode"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestAuthMiddleware 單獨測驗證層——這是「兩層測試」的另一層：
// handler 測試走完整 middleware 鏈（整合視角），middleware 自己則用
// 裸的 gin router 只掛它一個來測（單元視角），互不干擾。
// 測試路由的 handler 回傳 context 裡的登入者，驗證放行 / 中斷行為。
func TestAuthMiddleware(t *testing.T) {
	authUser := AuthUser{UserID: 1, Username: "alice", Email: "alice@example.com"}

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, cacheMock *mockcache.MockCache)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			setupAuth: func(t *testing.T, request *http.Request, cacheMock *mockcache.MockCache) {
				setupAuth(t, request, cacheMock, authUser)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				// 放行後 handler 拿到的就是 session 裡的登入者
				var got AuthUser
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &got))
				require.Equal(t, authUser, got)
			},
		},
		{
			name: "NoToken",
			setupAuth: func(t *testing.T, request *http.Request, cacheMock *mockcache.MockCache) {
				// 不帶 token：直接 401，不會打到 cache（沒排 Get 的劇本也不會報錯）
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
				require.Equal(t, errcode.ErrUnauthorized, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "SessionNotFound",
			setupAuth: func(t *testing.T, request *http.Request, cacheMock *mockcache.MockCache) {
				// token 有帶，但 Redis 上沒有（過期或亂給）→ 401
				token := "expired-token"
				cacheMock.EXPECT().
					Get(gomock.Any(), gomock.Eq(sessionKey(token))).
					Times(1).
					Return("", cache.ErrNotFound)
				request.Header.Set(tokenHeaderKey, token)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
				require.Equal(t, errcode.ErrUnauthorized, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "CacheError",
			setupAuth: func(t *testing.T, request *http.Request, cacheMock *mockcache.MockCache) {
				// Redis 掛了是系統錯誤，不能誤報成「未登入」→ 500
				token := "any-token"
				cacheMock.EXPECT().
					Get(gomock.Any(), gomock.Eq(sessionKey(token))).
					Times(1).
					Return("", errors.New("redis is down"))
				request.Header.Set(tokenHeaderKey, token)
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
			cacheMock := mockcache.NewMockCache(ctrl)

			// 裸 router：只掛被測的 middleware，不經過完整鏈
			router := gin.New()
			router.GET("/auth", authMiddleware(cacheMock), func(ctx *gin.Context) {
				ok(ctx, getAuthUser(ctx))
			})

			recorder := httptest.NewRecorder()
			request, err := http.NewRequest(http.MethodGet, "/auth", nil)
			require.NoError(t, err)
			tc.setupAuth(t, request, cacheMock)

			router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
