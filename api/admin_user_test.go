package api

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	mockcache "github.com/kys20548/template_golang_web/cache/mock"
	mockdb "github.com/kys20548/template_golang_web/db/mock"
	db "github.com/kys20548/template_golang_web/db/sqlc"
	"github.com/kys20548/template_golang_web/errcode"
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
					ListAdminUsers(gomock.Any(), gomock.Eq(db.ListAdminUsersParams{Limit: 10, Offset: 0})).
					Times(1).
					Return(adminUsers, nil)
				store.EXPECT().
					CountAdminUsers(gomock.Any()).
					Times(1).
					Return(int64(2), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var got struct {
					Total int64               `json:"total"`
					List  []adminUserResponse `json:"list"`
				}
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &got))
				require.Equal(t, int64(2), got.Total)
				require.Len(t, got.List, 2)
				require.Equal(t, newAdminUserResponse(adminUsers[0]), got.List[0])
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
