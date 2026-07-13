package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	mockcache "github.com/kys20548/template_golang_web/cache/mock"
	mockdb "github.com/kys20548/template_golang_web/db/mock"
	"github.com/kys20548/template_golang_web/errcode"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestReadyCheckAPI(t *testing.T) {
	testCases := []struct {
		name          string
		buildStubs    func(store *mockdb.MockStore, cacheMock *mockcache.MockCache)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			buildStubs: func(store *mockdb.MockStore, cacheMock *mockcache.MockCache) {
				store.EXPECT().Ping(gomock.Any()).Times(1).Return(nil)
				cacheMock.EXPECT().Ping(gomock.Any()).Times(1).Return(nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var data string
				require.Equal(t, errcode.Success, parseResponse(t, recorder.Body, &data))
				require.Equal(t, "ready", data)
			},
		},
		{
			name: "DBDown",
			buildStubs: func(store *mockdb.MockStore, cacheMock *mockcache.MockCache) {
				store.EXPECT().Ping(gomock.Any()).Times(1).Return(errors.New("connection refused"))
				// DB 掛了就直接回 503，不該再去 ping Redis
				cacheMock.EXPECT().Ping(gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
				require.Equal(t, errcode.ErrNotReady, parseResponse(t, recorder.Body, nil))
			},
		},
		{
			name: "RedisDown",
			buildStubs: func(store *mockdb.MockStore, cacheMock *mockcache.MockCache) {
				store.EXPECT().Ping(gomock.Any()).Times(1).Return(nil)
				cacheMock.EXPECT().Ping(gomock.Any()).Times(1).Return(errors.New("connection refused"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
				require.Equal(t, errcode.ErrNotReady, parseResponse(t, recorder.Body, nil))
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

			request, err := http.NewRequest(http.MethodGet, "/readyz", nil)
			require.NoError(t, err)

			server.Router().ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
