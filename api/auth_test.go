package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kys20548/template_golang_web/cache"
	mockcache "github.com/kys20548/template_golang_web/cache/mock"
	mockdb "github.com/kys20548/template_golang_web/db/mock"
	db "github.com/kys20548/template_golang_web/db/sqlc"
	"github.com/kys20548/template_golang_web/errcode"
	"github.com/kys20548/template_golang_web/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestLoginAPI 是 mock cache 用得最充分的測試：
// 登入流程對 cache 的每一步互動（查失敗計數、記失敗、清計數、寫 session）
// 都是業務邏輯的一部分，mock 的 Times(n) 能逐一驗證。
func TestLoginAPI(t *testing.T) {
	user, password := testUser(t)

	testCases := []struct {
		name          string
		body          gin.H
		buildStubs    func(store *mockdb.MockStore, cacheMock *mockcache.MockCache)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{
				"username": user.Username,
				"password": password,
			},
			buildStubs: func(store *mockdb.MockStore, cacheMock *mockcache.MockCache) {
				// 失敗計數不存在（沒失敗過）
				cacheMock.EXPECT().
					Get(gomock.Any(), gomock.Eq(loginFailKey(user.Username))).
					Times(1).
					Return("", cache.ErrNotFound)
				store.EXPECT().
					GetUserByUsername(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(user, nil)
				// 登入成功：清除失敗計數 + 寫入 session
				// （token 是隨機 UUID，key 無法預測，用 Any）
				cacheMock.EXPECT().
					Del(gomock.Any(), gomock.Eq(loginFailKey(user.Username))).
					Times(1).
					Return(nil)
				cacheMock.EXPECT().
					Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
				store.EXPECT().
					CreateOperationLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OperationLog{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var got loginResponse
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &got))
				require.NotEmpty(t, got.Token)
				require.Equal(t, newUserResponse(user), got.User)
			},
		},
		{
			name: "UserNotFound",
			body: gin.H{
				"username": user.Username,
				"password": password,
			},
			buildStubs: func(store *mockdb.MockStore, cacheMock *mockcache.MockCache) {
				cacheMock.EXPECT().
					Get(gomock.Any(), gomock.Eq(loginFailKey(user.Username))).
					Times(1).
					Return("", cache.ErrNotFound)
				store.EXPECT().
					GetUserByUsername(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.User{}, sql.ErrNoRows)
				// 帳號不存在也要記一次失敗——跟密碼錯誤走同一條路，
				// 避免撞庫時從行為差異洩漏帳號是否存在
				cacheMock.EXPECT().
					Incr(gomock.Any(), gomock.Eq(loginFailKey(user.Username)), gomock.Eq(loginFailTTL)).
					Times(1).
					Return(int64(1), nil)
				store.EXPECT().
					CreateOperationLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OperationLog{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
				require.Equal(t, errcode.ErrWrongCredentials, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "WrongPassword",
			body: gin.H{
				"username": user.Username,
				"password": "wrongpassword",
			},
			buildStubs: func(store *mockdb.MockStore, cacheMock *mockcache.MockCache) {
				cacheMock.EXPECT().
					Get(gomock.Any(), gomock.Eq(loginFailKey(user.Username))).
					Times(1).
					Return("", cache.ErrNotFound)
				// user 存在，但接下來 CheckPassword 會失敗（真的 bcrypt 比對）
				store.EXPECT().
					GetUserByUsername(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(user, nil)
				cacheMock.EXPECT().
					Incr(gomock.Any(), gomock.Eq(loginFailKey(user.Username)), gomock.Eq(loginFailTTL)).
					Times(1).
					Return(int64(1), nil)
				store.EXPECT().
					CreateOperationLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OperationLog{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
				require.Equal(t, errcode.ErrWrongCredentials, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "TooManyLoginFails",
			body: gin.H{
				"username": user.Username,
				"password": password,
			},
			buildStubs: func(store *mockdb.MockStore, cacheMock *mockcache.MockCache) {
				// 失敗計數已達上限 → 直接鎖定，不查 DB
				cacheMock.EXPECT().
					Get(gomock.Any(), gomock.Eq(loginFailKey(user.Username))).
					Times(1).
					Return("5", nil)
				store.EXPECT().
					GetUserByUsername(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					CreateOperationLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OperationLog{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusTooManyRequests, recorder.Code)
				require.Equal(t, errcode.ErrTooManyLoginFails, parseResponse(t, recorder.Body, nil))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := mockdb.NewMockStore(ctrl)
			cacheMock := mockcache.NewMockCache(ctrl)
			tc.buildStubs(store, cacheMock)

			server := newTestServer(t, store, cacheMock)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPost, "/login", bytes.NewReader(data))
			require.NoError(t, err)

			server.Router().ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestLogoutAPI(t *testing.T) {
	user, _ := testUser(t)

	ctrl := gomock.NewController(t)
	store := mockdb.NewMockStore(ctrl)
	cacheMock := mockcache.NewMockCache(ctrl)

	// 登出 = 刪掉 Redis 上的 session（setupAuth 用的 token 是 fake-token）
	cacheMock.EXPECT().
		Del(gomock.Any(), gomock.Eq(sessionKey("fake-token"))).
		Times(1).
		Return(nil)
	store.EXPECT().
		CreateOperationLog(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.OperationLog{}, nil)

	server := newTestServer(t, store, cacheMock)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodPost, "/logout", nil)
	require.NoError(t, err)
	setupAuth(t, request, cacheMock, toAuthUser(user))

	server.Router().ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, nil))
}

func TestMeAPI(t *testing.T) {
	user, _ := testUser(t)

	ctrl := gomock.NewController(t)
	store := mockdb.NewMockStore(ctrl)
	cacheMock := mockcache.NewMockCache(ctrl)

	server := newTestServer(t, store, cacheMock)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/me", nil)
	require.NoError(t, err)
	setupAuth(t, request, cacheMock, toAuthUser(user))

	server.Router().ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	// /me 不碰 DB，回的就是 session 裡的登入者資訊
	var got AuthUser
	require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &got))
	require.Equal(t, toAuthUser(user), got)
}

// eqUpdateUserPasswordParams：同 eqCreateUserTxParams 的思路——
// bcrypt 雜湊無法預測，用 CheckPassword 驗證雜湊來源後再比對其餘欄位。
type eqUpdateUserPasswordParamsMatcher struct {
	userID   int64
	password string
}

func (e eqUpdateUserPasswordParamsMatcher) Matches(x any) bool {
	arg, ok := x.(db.UpdateUserPasswordParams)
	if !ok {
		return false
	}
	if arg.ID != e.userID {
		return false
	}
	return util.CheckPassword(e.password, arg.HashedPassword) == nil
}

func (e eqUpdateUserPasswordParamsMatcher) String() string {
	return fmt.Sprintf("matches user id %d and new password %v", e.userID, e.password)
}

func eqUpdateUserPasswordParams(userID int64, password string) gomock.Matcher {
	return eqUpdateUserPasswordParamsMatcher{userID: userID, password: password}
}

func TestChangePasswordAPI(t *testing.T) {
	user, password := testUser(t)
	newPassword := "newsecret456"

	testCases := []struct {
		name          string
		body          gin.H
		buildStubs    func(store *mockdb.MockStore, cacheMock *mockcache.MockCache)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{"old_password": password, "new_password": newPassword},
			buildStubs: func(store *mockdb.MockStore, cacheMock *mockcache.MockCache) {
				store.EXPECT().
					GetUser(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(user, nil)
				store.EXPECT().
					UpdateUserPassword(gomock.Any(), eqUpdateUserPasswordParams(user.ID, newPassword)).
					Times(1).
					Return(nil)
				// 改完密碼刪掉目前的 session，強制重新登入
				cacheMock.EXPECT().
					Del(gomock.Any(), gomock.Eq(sessionKey("fake-token"))).
					Times(1).
					Return(nil)
				store.EXPECT().
					CreateOperationLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OperationLog{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "WrongOldPassword",
			body: gin.H{"old_password": "wrong-password", "new_password": newPassword},
			buildStubs: func(store *mockdb.MockStore, cacheMock *mockcache.MockCache) {
				store.EXPECT().
					GetUser(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(user, nil)
				// 舊密碼錯：不能改密碼、不能動 session
				store.EXPECT().UpdateUserPassword(gomock.Any(), gomock.Any()).Times(0)
				cacheMock.EXPECT().Del(gomock.Any(), gomock.Any()).Times(0)
				store.EXPECT().
					CreateOperationLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OperationLog{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Equal(t, errcode.ErrWrongCredentials, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "NewPasswordTooShort",
			body: gin.H{"old_password": password, "new_password": "123"},
			buildStubs: func(store *mockdb.MockStore, cacheMock *mockcache.MockCache) {
				// 參數驗證失敗就不該碰 DB
				store.EXPECT().GetUser(gomock.Any(), gomock.Any()).Times(0)
				store.EXPECT().
					CreateOperationLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OperationLog{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Equal(t, errcode.ErrInvalidParams, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "UpdateFails",
			body: gin.H{"old_password": password, "new_password": newPassword},
			buildStubs: func(store *mockdb.MockStore, cacheMock *mockcache.MockCache) {
				store.EXPECT().
					GetUser(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(user, nil)
				store.EXPECT().
					UpdateUserPassword(gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("db is down"))
				// 密碼沒改成，session 不能動
				cacheMock.EXPECT().Del(gomock.Any(), gomock.Any()).Times(0)
				store.EXPECT().
					CreateOperationLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OperationLog{}, nil)
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
			tc.buildStubs(store, cacheMock)

			server := newTestServer(t, store, cacheMock)
			recorder := httptest.NewRecorder()

			body, err := json.Marshal(tc.body)
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPut, "/me/password", bytes.NewReader(body))
			require.NoError(t, err)
			setupAuth(t, request, cacheMock, toAuthUser(user))

			server.Router().ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
