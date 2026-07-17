package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kys20548/template_golang_web/cache"
	mockcache "github.com/kys20548/template_golang_web/cache/mock"
	mockdb "github.com/kys20548/template_golang_web/db/mock"
	db "github.com/kys20548/template_golang_web/db/sqlc"
	"github.com/kys20548/template_golang_web/errcode"
	"github.com/kys20548/template_golang_web/util"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestListAdminUsersAPI(t *testing.T) {
	adminUser, _ := testAdminUser(t)
	adminUsers := []db.AdminUser{
		{ID: 1, Username: "admin"},
		{ID: 2, Username: "operator"},
	}

	testCases := []struct {
		name          string
		url           string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			url:  "/admin-users?pageNum=1&pageSize=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListAdminUsers(gomock.Any(), gomock.Eq(db.ListAdminUsersParams{PageLimit: 10, PageOffset: 0})).
					Times(1).
					Return(adminUsers, nil)
				store.EXPECT().
					CountAdminUsers(gomock.Any(), gomock.Eq(false)).
					Times(1).
					Return(int64(2), nil)
				// 角色關聯一次撈全部，handler 在記憶體組裝；operator 沒角色
				store.EXPECT().
					ListAdminUserRoles(gomock.Any()).
					Times(1).
					Return([]db.ListAdminUserRolesRow{
						{AdminUserID: 1, RoleID: 1, Name: "super_admin"},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var got struct {
					Total int64                        `json:"total"`
					List  []adminUserWithRolesResponse `json:"list"`
				}
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &got))
				require.Equal(t, int64(2), got.Total)
				require.Len(t, got.List, 2)
				require.Equal(t, newAdminUserResponse(adminUsers[0]), got.List[0].adminUserResponse)
				require.Equal(t, []roleBrief{{ID: 1, Name: "super_admin"}}, got.List[0].Roles)
				// 沒角色的帳號回空陣列而不是 null
				require.Equal(t, []roleBrief{}, got.List[1].Roles)
			},
		},
		{
			name: "InvalidPageNum",
			url:  "/admin-users?pageNum=0&pageSize=10", // binding min=1
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().ListAdminUsers(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Equal(t, errcode.ErrInvalidParams, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "InternalError",
			url:  "/admin-users?pageNum=1&pageSize=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListAdminUsers(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil, sql.ErrConnDone)
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
			setupAuth(t, request, cacheMock, toAuthUser(adminUser))

			server.Router().ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestPermMiddleware 驗證權限層：權限快照不含該資源的 code 就 403，
// 帶了對的 code 或萬用 * 才放行。403 要短路——不能打到 DB。
func TestPermMiddleware(t *testing.T) {
	adminUser, _ := testAdminUser(t)

	testCases := []struct {
		name          string
		permissions   []string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:        "NoPermission",
			permissions: []string{"user:read"}, // 有前台查詢權限，但這裡打的是 /admin-users
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().ListAdminUsers(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
				require.Equal(t, errcode.ErrForbidden, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name:        "ExactPermission",
			permissions: []string{"admin_user:read"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListAdminUsers(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.AdminUser{}, nil)
				store.EXPECT().CountAdminUsers(gomock.Any(), gomock.Any()).Times(1).Return(int64(0), nil)
				store.EXPECT().ListAdminUserRoles(gomock.Any()).Times(1).Return(nil, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:        "Wildcard",
			permissions: []string{"*"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListAdminUsers(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.AdminUser{}, nil)
				store.EXPECT().CountAdminUsers(gomock.Any(), gomock.Any()).Times(1).Return(int64(0), nil)
				store.EXPECT().ListAdminUserRoles(gomock.Any()).Times(1).Return(nil, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
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

			request, err := http.NewRequest(http.MethodGet, "/admin-users?pageNum=1&pageSize=10", nil)
			require.NoError(t, err)
			setupAuth(t, request, cacheMock, toAuthUserWithPerms(adminUser, tc.permissions))

			server.Router().ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// eqCreateAdminUserTxParams：bcrypt 雜湊無法預測，用 CheckPassword
// 驗證雜湊來源後再比對其餘欄位（同 eqCreateUserTxParams 的思路）。
type eqCreateAdminUserTxParamsMatcher struct {
	username string
	password string
	roleIDs  []int64
}

func (e eqCreateAdminUserTxParamsMatcher) Matches(x any) bool {
	arg, ok := x.(db.CreateAdminUserTxParams)
	if !ok {
		return false
	}
	if arg.Username != e.username {
		return false
	}
	if !slices.Equal(arg.RoleIDs, e.roleIDs) {
		return false
	}
	return util.CheckPassword(e.password, arg.HashedPassword) == nil
}

func (e eqCreateAdminUserTxParamsMatcher) String() string {
	return fmt.Sprintf("matches username %s, password %s and role ids %v", e.username, e.password, e.roleIDs)
}

func eqCreateAdminUserTxParams(username, password string, roleIDs []int64) gomock.Matcher {
	return eqCreateAdminUserTxParamsMatcher{username: username, password: password, roleIDs: roleIDs}
}

func TestCreateAdminUserAPI(t *testing.T) {
	adminUser, _ := testAdminUser(t)

	testCases := []struct {
		name          string
		body          gin.H
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{"username": "operator", "password": "secret123", "role_ids": []int64{2}},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					CreateAdminUserTx(gomock.Any(), eqCreateAdminUserTxParams("operator", "secret123", []int64{2})).
					Times(1).
					Return(db.CreateAdminUserTxResult{AdminUser: db.AdminUser{ID: 2, Username: "operator"}}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var got adminUserResponse
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &got))
				require.Equal(t, "operator", got.Username)
			},
		},
		{
			name: "DuplicateUsername",
			body: gin.H{"username": "operator", "password": "secret123"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					CreateAdminUserTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateAdminUserTxResult{}, &pq.Error{Code: "23505"}) // unique_violation
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
				require.Equal(t, errcode.ErrUserExists, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "RoleNotFound",
			body: gin.H{"username": "operator", "password": "secret123", "role_ids": []int64{999}},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					CreateAdminUserTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateAdminUserTxResult{}, &pq.Error{Code: "23503"}) // foreign_key_violation
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Equal(t, errcode.ErrInvalidParams, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "PasswordTooShort",
			body: gin.H{"username": "operator", "password": "123"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().CreateAdminUserTx(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Equal(t, errcode.ErrInvalidParams, parseResponse(t, recorder.Body, nil))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := mockdb.NewMockStore(ctrl)
			cacheMock := mockcache.NewMockCache(ctrl)
			tc.buildStubs(store)
			// POST 會被 audit middleware 記一筆操作日誌
			store.EXPECT().
				CreateOperationLog(gomock.Any(), gomock.Any()).
				Times(1).
				Return(db.OperationLog{}, nil)

			server := newTestServer(t, store, cacheMock)
			recorder := httptest.NewRecorder()

			body, err := json.Marshal(tc.body)
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPost, "/admin-users", bytes.NewReader(body))
			require.NoError(t, err)
			setupAuth(t, request, cacheMock, toAuthUser(adminUser))

			server.Router().ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestUpdateAdminUserRolesAPI(t *testing.T) {
	adminUser, _ := testAdminUser(t)

	testCases := []struct {
		name          string
		url           string
		body          gin.H
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			url:  "/admin-users/2/roles",
			body: gin.H{"role_ids": []int64{1, 2}},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetAdminUser(gomock.Any(), gomock.Eq(int64(2))).
					Times(1).
					Return(db.AdminUser{ID: 2, Username: "operator"}, nil)
				store.EXPECT().
					UpdateAdminUserRolesTx(gomock.Any(), gomock.Eq(db.UpdateAdminUserRolesTxParams{
						AdminUserID: 2,
						RoleIDs:     []int64{1, 2},
					})).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "UserNotFound",
			url:  "/admin-users/999/roles",
			body: gin.H{"role_ids": []int64{1}},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetAdminUser(gomock.Any(), gomock.Eq(int64(999))).
					Times(1).
					Return(db.AdminUser{}, sql.ErrNoRows)
				store.EXPECT().UpdateAdminUserRolesTx(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
				require.Equal(t, errcode.ErrUserNotFound, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "RoleNotFound",
			url:  "/admin-users/2/roles",
			body: gin.H{"role_ids": []int64{999}},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetAdminUser(gomock.Any(), gomock.Eq(int64(2))).
					Times(1).
					Return(db.AdminUser{ID: 2, Username: "operator"}, nil)
				store.EXPECT().
					UpdateAdminUserRolesTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(&pq.Error{Code: "23503"}) // foreign_key_violation
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Equal(t, errcode.ErrInvalidParams, parseResponse(t, recorder.Body, nil))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := mockdb.NewMockStore(ctrl)
			cacheMock := mockcache.NewMockCache(ctrl)
			tc.buildStubs(store)
			store.EXPECT().
				CreateOperationLog(gomock.Any(), gomock.Any()).
				Times(1).
				Return(db.OperationLog{}, nil)

			server := newTestServer(t, store, cacheMock)
			recorder := httptest.NewRecorder()

			body, err := json.Marshal(tc.body)
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPut, tc.url, bytes.NewReader(body))
			require.NoError(t, err)
			setupAuth(t, request, cacheMock, toAuthUser(adminUser))

			server.Router().ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListRolesAPI(t *testing.T) {
	adminUser, _ := testAdminUser(t)

	ctrl := gomock.NewController(t)
	store := mockdb.NewMockStore(ctrl)
	cacheMock := mockcache.NewMockCache(ctrl)

	store.EXPECT().
		ListRoles(gomock.Any()).
		Times(1).
		Return([]db.Role{
			{ID: 1, Name: "super_admin", Description: "超級管理員（全部權限）"},
			{ID: 2, Name: "viewer", Description: "唯讀"},
		}, nil)
	store.EXPECT().
		ListRolePermissions(gomock.Any()).
		Times(1).
		Return([]db.ListRolePermissionsRow{
			{RoleID: 1, Code: "*"},
			{RoleID: 2, Code: "operation_log:read"},
			{RoleID: 2, Code: "user:read"},
		}, nil)

	server := newTestServer(t, store, cacheMock)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/roles", nil)
	require.NoError(t, err)
	setupAuth(t, request, cacheMock, toAuthUser(adminUser))

	server.Router().ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var got []roleResponse
	require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &got))
	require.Len(t, got, 2)
	require.Equal(t, []string{"*"}, got[0].Permissions)
	require.Equal(t, []string{"operation_log:read", "user:read"}, got[1].Permissions)
}

func TestDeleteAdminUserAPI(t *testing.T) {
	adminUser, _ := testAdminUser(t) // 操作者 ID=1

	testCases := []struct {
		name          string
		url           string
		buildStubs    func(store *mockdb.MockStore, cacheMock *mockcache.MockCache)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			// 刪除成功後要透過反查索引把對方 session 踢下線
			name: "OKAndKicksSession",
			url:  "/admin-users/2",
			buildStubs: func(store *mockdb.MockStore, cacheMock *mockcache.MockCache) {
				store.EXPECT().
					SoftDeleteAdminUser(gomock.Any(), gomock.Eq(int64(2))).
					Times(1).
					Return(int64(1), nil)
				cacheMock.EXPECT().
					Get(gomock.Any(), gomock.Eq(adminSessionKey(2))).
					Times(1).
					Return("victim-token", nil)
				cacheMock.EXPECT().
					Del(gomock.Any(), gomock.Eq(sessionKey("victim-token")), gomock.Eq(adminSessionKey(2))).
					Times(1).
					Return(nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			// 對方沒登入（反查索引不存在）：不算錯，正常回成功
			name: "OKNoActiveSession",
			url:  "/admin-users/2",
			buildStubs: func(store *mockdb.MockStore, cacheMock *mockcache.MockCache) {
				store.EXPECT().
					SoftDeleteAdminUser(gomock.Any(), gomock.Eq(int64(2))).
					Times(1).
					Return(int64(1), nil)
				cacheMock.EXPECT().
					Get(gomock.Any(), gomock.Eq(adminSessionKey(2))).
					Times(1).
					Return("", cache.ErrNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			// 不能刪除自己（操作者 ID=1）：參數檢查就擋掉，不碰 DB
			name: "CannotDeleteSelf",
			url:  "/admin-users/1",
			buildStubs: func(store *mockdb.MockStore, cacheMock *mockcache.MockCache) {
				store.EXPECT().SoftDeleteAdminUser(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Equal(t, errcode.ErrCannotDeleteSelf, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "NotFound",
			url:  "/admin-users/999",
			buildStubs: func(store *mockdb.MockStore, cacheMock *mockcache.MockCache) {
				store.EXPECT().
					SoftDeleteAdminUser(gomock.Any(), gomock.Eq(int64(999))).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
				require.Equal(t, errcode.ErrUserNotFound, parseResponse(t, recorder.Body, nil))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := mockdb.NewMockStore(ctrl)
			cacheMock := mockcache.NewMockCache(ctrl)
			tc.buildStubs(store, cacheMock)
			// DELETE 會被 audit middleware 記操作日誌
			store.EXPECT().
				CreateOperationLog(gomock.Any(), gomock.Any()).
				Times(1).
				Return(db.OperationLog{}, nil)

			server := newTestServer(t, store, cacheMock)
			recorder := httptest.NewRecorder()

			request, err := http.NewRequest(http.MethodDelete, tc.url, nil)
			require.NoError(t, err)
			setupAuth(t, request, cacheMock, toAuthUser(adminUser))

			server.Router().ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestRestoreAdminUserAPI(t *testing.T) {
	adminUser, _ := testAdminUser(t)

	testCases := []struct {
		name          string
		url           string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			url:  "/admin-users/2/restore",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					RestoreAdminUser(gomock.Any(), gomock.Eq(int64(2))).
					Times(1).
					Return(int64(1), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "NotFound",
			url:  "/admin-users/999/restore",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					RestoreAdminUser(gomock.Any(), gomock.Eq(int64(999))).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
				require.Equal(t, errcode.ErrUserNotFound, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			// 刪除期間同名帳號被重新建立：還原撞 partial unique index → 409
			name: "UsernameRetaken",
			url:  "/admin-users/2/restore",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					RestoreAdminUser(gomock.Any(), gomock.Eq(int64(2))).
					Times(1).
					Return(int64(0), &pq.Error{Code: "23505"}) // unique_violation
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
				require.Equal(t, errcode.ErrUserExists, parseResponse(t, recorder.Body, nil))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := mockdb.NewMockStore(ctrl)
			cacheMock := mockcache.NewMockCache(ctrl)
			tc.buildStubs(store)
			// PUT 會被 audit middleware 記操作日誌
			store.EXPECT().
				CreateOperationLog(gomock.Any(), gomock.Any()).
				Times(1).
				Return(db.OperationLog{}, nil)

			server := newTestServer(t, store, cacheMock)
			recorder := httptest.NewRecorder()

			request, err := http.NewRequest(http.MethodPut, tc.url, nil)
			require.NoError(t, err)
			setupAuth(t, request, cacheMock, toAuthUser(adminUser))

			server.Router().ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
