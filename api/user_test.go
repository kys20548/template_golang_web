package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	mockcache "github.com/kys20548/template_golang_web/cache/mock"
	mockdb "github.com/kys20548/template_golang_web/db/mock"
	db "github.com/kys20548/template_golang_web/db/sqlc"
	"github.com/kys20548/template_golang_web/errcode"
	"github.com/kys20548/template_golang_web/util"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ───────────────────────────────────────────────────────────────
// 入門示範：先不碰 HTTP，單純看 mock 是什麼。
//
// mockgen 讀了 Store interface 後，產生一個 MockStore struct，
// 它實作了 Store 的所有方法——但方法內容不是查 DB，而是
// 「回傳你事先用 EXPECT() 排好的劇本」。
// ───────────────────────────────────────────────────────────────
func TestMockStoreDemo(t *testing.T) {
	// Controller 是 gomock 的裁判：測試結束時檢查
	// 「劇本裡排的呼叫是不是真的都發生了」，沒發生就讓測試失敗。
	ctrl := gomock.NewController(t)
	store := mockdb.NewMockStore(ctrl)

	// 排劇本：「GetUser 會被用 id=7 呼叫恰好 1 次，到時回傳這個 user、error 為 nil」
	store.EXPECT().
		GetUser(gomock.Any(), int64(7)). // 第一個參數是 context，內容不重要，用 Any
		Times(1).
		Return(db.User{ID: 7, Username: "alice"}, nil)

	// 真的呼叫看看——沒有 DB、沒有網路，拿到的就是劇本裡的值
	user, err := store.GetUser(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, "alice", user.Username)
}

// healthz 不需要任何依賴，是最單純的 handler 測試。
func TestHealthzAPI(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := newTestServer(t, mockdb.NewMockStore(ctrl), mockcache.NewMockCache(ctrl))

	recorder := httptest.NewRecorder()
	request, err := http.NewRequest(http.MethodGet, "/healthz", nil)
	require.NoError(t, err)

	server.Router().ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, nil))
}

// TestGetUserAPI 是 table-driven 測試：同一支 API 的所有情境列成一張表，
// 每個 case 由三件事組成——怎麼登入（setupAuth）、DB 劇本（buildStubs）、
// 驗什麼（checkResponse）；發請求的流程共用最下面那一段。
func TestGetUserAPI(t *testing.T) {
	user, _ := testUser(t)

	testCases := []struct {
		name          string
		url           string
		setupAuth     func(t *testing.T, request *http.Request, cacheMock *mockcache.MockCache)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			url:  fmt.Sprintf("/users/%d", user.ID),
			setupAuth: func(t *testing.T, request *http.Request, cacheMock *mockcache.MockCache) {
				setupAuth(t, request, cacheMock, toAuthUser(user))
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUser(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(user, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var got userResponse
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &got))
				require.Equal(t, newUserResponse(user), got)
			},
		},
		{
			name: "NotFound",
			url:  fmt.Sprintf("/users/%d", user.ID),
			setupAuth: func(t *testing.T, request *http.Request, cacheMock *mockcache.MockCache) {
				setupAuth(t, request, cacheMock, toAuthUser(user))
			},
			buildStubs: func(store *mockdb.MockStore) {
				// handler 對 sql.ErrNoRows 要轉成 404 + 業務碼 20001
				store.EXPECT().
					GetUser(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.User{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
				require.Equal(t, errcode.ErrUserNotFound, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "InternalError",
			url:  fmt.Sprintf("/users/%d", user.ID),
			setupAuth: func(t *testing.T, request *http.Request, cacheMock *mockcache.MockCache) {
				setupAuth(t, request, cacheMock, toAuthUser(user))
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 其他 DB 錯誤走 failInternal → 500 + 10001
				store.EXPECT().
					GetUser(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.User{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
				require.Equal(t, errcode.ErrInternal, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "InvalidID",
			url:  "/users/0", // binding min=1，參數驗證就擋掉
			setupAuth: func(t *testing.T, request *http.Request, cacheMock *mockcache.MockCache) {
				setupAuth(t, request, cacheMock, toAuthUser(user))
			},
			buildStubs: func(store *mockdb.MockStore) {
				// Times(0)：參數不合法就不該打 DB——短路行為也是被驗證的
				store.EXPECT().
					GetUser(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Equal(t, errcode.ErrInvalidParams, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "NoToken",
			url:  fmt.Sprintf("/users/%d", user.ID),
			setupAuth: func(t *testing.T, request *http.Request, cacheMock *mockcache.MockCache) {
				// 不帶 token：authMiddleware 直接擋下，連 cache 都不會查
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUser(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
				require.Equal(t, errcode.ErrUnauthorized, parseResponse(t, recorder.Body, nil))
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

			request, err := http.NewRequest(http.MethodGet, tc.url, nil)
			require.NoError(t, err)
			tc.setupAuth(t, request, cacheMock)

			server.Router().ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ───────────────────────────────────────────────────────────────
// eqCreateUserTxParams：自訂 gomock matcher。
//
// bcrypt 每次雜湊同一個密碼結果都不同，所以 handler 傳給 DB 的
// HashedPassword 無法事先預測，不能用 gomock.Eq 整包比對。
// 解法：用 CheckPassword 驗證「這個雜湊確實來自預期的明文密碼」，
// 其餘欄位再逐一比對。
// ───────────────────────────────────────────────────────────────
type eqCreateUserTxParamsMatcher struct {
	arg      db.CreateUserTxParams
	password string
}

func (e eqCreateUserTxParamsMatcher) Matches(x any) bool {
	arg, ok := x.(db.CreateUserTxParams)
	if !ok {
		return false
	}

	if err := util.CheckPassword(e.password, arg.HashedPassword); err != nil {
		return false
	}

	e.arg.HashedPassword = arg.HashedPassword
	return reflect.DeepEqual(e.arg, arg)
}

func (e eqCreateUserTxParamsMatcher) String() string {
	return fmt.Sprintf("matches arg %v and password %v", e.arg, e.password)
}

func eqCreateUserTxParams(arg db.CreateUserTxParams, password string) gomock.Matcher {
	return eqCreateUserTxParamsMatcher{arg: arg, password: password}
}

func TestCreateUserAPI(t *testing.T) {
	user, password := testUser(t)
	wallet := db.Wallet{ID: 3, UserID: user.ID}

	testCases := []struct {
		name          string
		body          gin.H
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{
				"username": user.Username,
				"email":    user.Email,
				"password": password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.CreateUserTxParams{
					CreateUserParams: db.CreateUserParams{
						Username: user.Username,
						Email:    user.Email,
					},
				}
				store.EXPECT().
					CreateUserTx(gomock.Any(), eqCreateUserTxParams(arg, password)).
					Times(1).
					Return(db.CreateUserTxResult{User: user, Wallet: wallet}, nil)
				// POST 會經過 auditLogMiddleware 寫一筆操作日誌，
				// 不排這筆劇本 gomock 會報 unexpected call
				store.EXPECT().
					CreateOperationLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OperationLog{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var got createUserResponse
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &got))
				require.Equal(t, newUserResponse(user), got.User)
				require.Equal(t, wallet, got.Wallet)
				// 敏感欄位不得外洩
				require.NotContains(t, recorder.Body.String(), "hashed_password")
			},
		},
		{
			name: "InvalidEmail",
			body: gin.H{
				"username": user.Username,
				"email":    "not-an-email",
				"password": password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					CreateUserTx(gomock.Any(), gomock.Any()).
					Times(0)
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
			name: "DuplicateUsername",
			body: gin.H{
				"username": user.Username,
				"email":    user.Email,
				"password": password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 模擬 PostgreSQL 的 unique constraint 錯誤（23505 = unique_violation）
				store.EXPECT().
					CreateUserTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateUserTxResult{}, &pq.Error{Code: "23505"})
				store.EXPECT().
					CreateOperationLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OperationLog{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
				require.Equal(t, errcode.ErrUserExists, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "InternalError",
			body: gin.H{
				"username": user.Username,
				"email":    user.Email,
				"password": password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					CreateUserTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateUserTxResult{}, sql.ErrConnDone)
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
			tc.buildStubs(store)

			server := newTestServer(t, store, cacheMock)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPost, "/users", bytes.NewReader(data))
			require.NoError(t, err)

			server.Router().ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListUsersAPI(t *testing.T) {
	user, _ := testUser(t)
	users := []db.User{
		{ID: 1, Username: "alice", Email: "alice@example.com"},
		{ID: 2, Username: "bob", Email: "bob@example.com"},
	}

	testCases := []struct {
		name          string
		url           string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			url:  "/users?pageNum=2&pageSize=5",
			buildStubs: func(store *mockdb.MockStore) {
				// 驗證分頁換算：pageNum=2, pageSize=5 → LIMIT 5 OFFSET 5
				store.EXPECT().
					ListUsers(gomock.Any(), gomock.Eq(db.ListUsersParams{Limit: 5, Offset: 5})).
					Times(1).
					Return(users, nil)
				store.EXPECT().
					CountUsers(gomock.Any()).
					Times(1).
					Return(int64(7), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var got PageResult
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &got))
				require.Equal(t, int32(2), got.PageNum)
				require.Equal(t, int32(5), got.PageSize)
				require.Equal(t, int64(7), got.Total)
			},
		},
		{
			name: "PageSizeTooLarge",
			url:  "/users?pageNum=1&pageSize=100", // binding max=50
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().ListUsers(gomock.Any(), gomock.Any()).Times(0)
				store.EXPECT().CountUsers(gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Equal(t, errcode.ErrInvalidParams, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "InternalError",
			url:  "/users?pageNum=1&pageSize=5",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUsers(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, sql.ErrConnDone)
				// 查列表就失敗了，不會再查 total
				store.EXPECT().CountUsers(gomock.Any()).Times(0)
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

			request, err := http.NewRequest(http.MethodGet, tc.url, nil)
			require.NoError(t, err)
			setupAuth(t, request, cacheMock, toAuthUser(user))

			server.Router().ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
