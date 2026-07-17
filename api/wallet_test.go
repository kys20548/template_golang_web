package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	mockcache "github.com/kys20548/template_golang_web/cache/mock"
	mockdb "github.com/kys20548/template_golang_web/db/mock"
	db "github.com/kys20548/template_golang_web/db/sqlc"
	"github.com/kys20548/template_golang_web/errcode"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestListWalletsAPI 測 GET /wallets——後台檢視所有前台 user 的錢包（分頁）。
func TestListWalletsAPI(t *testing.T) {
	adminUser, _ := testAdminUser(t)
	wallets := []db.ListWalletsRow{
		{ID: 1, UserID: 1, Username: "alice", Email: "alice@example.com", Balance: 1000},
		{ID: 2, UserID: 2, Username: "bob", Email: "bob@example.com", Balance: 0},
	}

	testCases := []struct {
		name          string
		url           string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			url:  "/wallets?pageNum=1&pageSize=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListWallets(gomock.Any(), gomock.Eq(db.ListWalletsParams{Limit: 10, Offset: 0})).
					Times(1).
					Return(wallets, nil)
				store.EXPECT().
					CountWallets(gomock.Any()).
					Times(1).
					Return(int64(2), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var got struct {
					PageNum  int32               `json:"pageNum"`
					PageSize int32               `json:"pageSize"`
					Total    int64               `json:"total"`
					List     []db.ListWalletsRow `json:"list"`
				}
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &got))
				require.Equal(t, int64(2), got.Total)
				require.Equal(t, wallets, got.List)
			},
		},
		{
			name: "InvalidPageSize",
			url:  "/wallets?pageNum=1&pageSize=100", // binding max=50
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().ListWallets(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Equal(t, errcode.ErrInvalidParams, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "InternalError",
			url:  "/wallets?pageNum=1&pageSize=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListWallets(gomock.Any(), gomock.Any()).
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

// TestGetWalletAPI 測 GET /wallets/:id——錢包明細頁的抬頭資訊。
func TestGetWalletAPI(t *testing.T) {
	adminUser, _ := testAdminUser(t)
	wallet := db.GetWalletDetailRow{
		ID: 1, UserID: 1, Username: "alice", Email: "alice@example.com", Balance: 1000,
	}

	testCases := []struct {
		name          string
		url           string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			url:  "/wallets/1",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetWalletDetail(gomock.Any(), gomock.Eq(int64(1))).
					Times(1).
					Return(wallet, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var got db.GetWalletDetailRow
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &got))
				require.Equal(t, wallet, got)
			},
		},
		{
			name: "NotFound",
			url:  "/wallets/999",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetWalletDetail(gomock.Any(), gomock.Eq(int64(999))).
					Times(1).
					Return(db.GetWalletDetailRow{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
				require.Equal(t, errcode.ErrNotFound, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "InvalidID",
			url:  "/wallets/abc",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().GetWalletDetail(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Equal(t, errcode.ErrInvalidParams, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "InternalError",
			url:  "/wallets/1",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetWalletDetail(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetWalletDetailRow{}, sql.ErrConnDone)
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

// TestAdjustWalletAPI 測 POST /wallets/:id/adjust——加扣款走 AdjustWalletTx，
// 餘額不足回 30001、錢包不存在回 404；操作者一律取自登入態。
func TestAdjustWalletAPI(t *testing.T) {
	adminUser, _ := testAdminUser(t)

	testCases := []struct {
		name          string
		url           string
		body          gin.H
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OKDeposit",
			url:  "/wallets/1/adjust",
			body: gin.H{"amount": 100, "note": "活動獎勵"},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.AdjustWalletTxParams{
					WalletID:         1,
					Amount:           100,
					Note:             "活動獎勵",
					OperatorID:       adminUser.ID,
					OperatorUsername: adminUser.Username,
				}
				store.EXPECT().
					AdjustWalletTx(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(db.AdjustWalletTxResult{
						Wallet: db.Wallet{ID: 1, UserID: 1, Balance: 1100},
						Entry:  db.WalletEntry{ID: 1, WalletID: 1, Amount: 100, Note: "活動獎勵"},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var got db.AdjustWalletTxResult
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &got))
				require.Equal(t, int64(1100), got.Wallet.Balance)
				require.Equal(t, int64(100), got.Entry.Amount)
			},
		},
		{
			name: "OKWithdraw",
			url:  "/wallets/1/adjust",
			body: gin.H{"amount": -100},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.AdjustWalletTxParams{
					WalletID:         1,
					Amount:           -100,
					OperatorID:       adminUser.ID,
					OperatorUsername: adminUser.Username,
				}
				store.EXPECT().
					AdjustWalletTx(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(db.AdjustWalletTxResult{
						Wallet: db.Wallet{ID: 1, UserID: 1, Balance: 900},
						Entry:  db.WalletEntry{ID: 2, WalletID: 1, Amount: -100},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var got db.AdjustWalletTxResult
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &got))
				require.Equal(t, int64(900), got.Wallet.Balance)
			},
		},
		{
			name: "InsufficientBalance",
			url:  "/wallets/1/adjust",
			body: gin.H{"amount": -99999},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					AdjustWalletTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.AdjustWalletTxResult{}, db.ErrInsufficientBalance)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Equal(t, errcode.ErrInsufficientBalance, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "WalletNotFound",
			url:  "/wallets/999/adjust",
			body: gin.H{"amount": 100},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					AdjustWalletTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.AdjustWalletTxResult{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
				require.Equal(t, errcode.ErrNotFound, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "ZeroAmount",
			url:  "/wallets/1/adjust",
			body: gin.H{"amount": 0},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().AdjustWalletTx(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Equal(t, errcode.ErrInvalidParams, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "InternalError",
			url:  "/wallets/1/adjust",
			body: gin.H{"amount": 100},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					AdjustWalletTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.AdjustWalletTxResult{}, sql.ErrConnDone)
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

			// POST 會經過 audit middleware 寫 operation log
			store.EXPECT().
				CreateOperationLog(gomock.Any(), gomock.Any()).
				Times(1).
				Return(db.OperationLog{}, nil)

			server := newTestServer(t, store, cacheMock)
			recorder := httptest.NewRecorder()

			body, err := json.Marshal(tc.body)
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPost, tc.url, bytes.NewReader(body))
			require.NoError(t, err)
			setupAuth(t, request, cacheMock, toAuthUser(adminUser))

			server.Router().ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestListWalletEntriesAPI 測 GET /wallets/:id/entries——單一錢包的異動明細（分頁）。
func TestListWalletEntriesAPI(t *testing.T) {
	adminUser, _ := testAdminUser(t)
	entries := []db.WalletEntry{
		{ID: 2, WalletID: 1, Amount: -50, Note: "扣款", OperatorID: 1, OperatorUsername: "admin"},
		{ID: 1, WalletID: 1, Amount: 100, Note: "加款", OperatorID: 1, OperatorUsername: "admin"},
	}

	testCases := []struct {
		name          string
		url           string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			url:  "/wallets/1/entries?pageNum=1&pageSize=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListWalletEntries(gomock.Any(), gomock.Eq(db.ListWalletEntriesParams{
						WalletID: 1, Limit: 10, Offset: 0,
					})).
					Times(1).
					Return(entries, nil)
				store.EXPECT().
					CountWalletEntries(gomock.Any(), gomock.Eq(int64(1))).
					Times(1).
					Return(int64(2), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var got struct {
					Total int64            `json:"total"`
					List  []db.WalletEntry `json:"list"`
				}
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &got))
				require.Equal(t, int64(2), got.Total)
				require.Equal(t, entries, got.List)
			},
		},
		{
			name: "InvalidPageSize",
			url:  "/wallets/1/entries?pageNum=1&pageSize=100", // binding max=50
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().ListWalletEntries(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.Equal(t, errcode.ErrInvalidParams, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "InternalError",
			url:  "/wallets/1/entries?pageNum=1&pageSize=10",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListWalletEntries(gomock.Any(), gomock.Any()).
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
