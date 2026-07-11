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

// TestGetMyWalletAPI 測 GET /wallet——「查自己的錢包」。
//
// 這支 API 示範專案的一條重要慣例：查「自己的」資源時，
// user 一律來自 context（登入 session），不接受 request 參數指定。
// 注意 request 沒有帶任何 user id——但 OK case 斷言 DB「必須」被用
// id=1 查詢，而 1 只可能來自 session。這證明 client 無法查別人的錢包。
func TestGetMyWalletAPI(t *testing.T) {
	user, _ := testUser(t)
	wallet := db.Wallet{ID: 3, UserID: user.ID, Balance: 1000}

	testCases := []struct {
		name          string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetWalletByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(wallet, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var got db.Wallet
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &got))
				require.Equal(t, wallet, got)
			},
		},
		{
			name: "NotFound",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetWalletByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Wallet{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
				require.Equal(t, errcode.ErrWalletNotFound, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "InternalError",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetWalletByUserID(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Wallet{}, sql.ErrConnDone)
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

			request, err := http.NewRequest(http.MethodGet, "/wallet", nil)
			require.NoError(t, err)
			setupAuth(t, request, cacheMock, toAuthUser(user))

			server.Router().ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
