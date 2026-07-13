package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kys20548/template_golang_web/cache"
	mockcache "github.com/kys20548/template_golang_web/cache/mock"
	db "github.com/kys20548/template_golang_web/db/sqlc"
	"github.com/kys20548/template_golang_web/errcode"
	"github.com/kys20548/template_golang_web/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestMain 是整個 package 測試的進入點（go test 會先跑它）。
// 把 gin 切到 TestMode，測試輸出才不會被 gin 的 debug 訊息洗版。
func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

// 這兩個 helper 把「會跟著基礎設施一起變的東西」集中在一個地方：
// 之後 NewServer 簽名變了、登入機制變了，只需要改這裡，
// 每個測試本身一個字都不用動。

// newTestServer 建立測試用 server：config 直接給值，不讀 app.env。
// APITimeout 一定要給：0 的話 timeoutMiddleware 會讓請求立刻超時。
func newTestServer(t *testing.T, store db.Store, cacheStore cache.Cache) *Server {
	config := util.Config{
		CORSAllowOrigins: "*",
		TokenDuration:    time.Minute,
		APITimeout:       10 * time.Second,
	}

	server, err := NewServer(config, store, cacheStore)
	require.NoError(t, err)

	return server
}

// setupAuth 讓一個請求變成「已登入」：
// 在 header 放 token，並排好 mock cache 的劇本——authMiddleware 來查
// session:<token> 時，回傳這個登入者的 JSON。
func setupAuth(t *testing.T, request *http.Request, cacheMock *mockcache.MockCache, user AuthUser) {
	token := "fake-token"

	payload, err := json.Marshal(user)
	require.NoError(t, err)

	cacheMock.EXPECT().
		Get(gomock.Any(), sessionKey(token)). // "session:fake-token"
		Times(1).
		Return(string(payload), nil)

	// sliding TTL：驗證通過後 authMiddleware 會續期 session，
	// 對測試本體不重要，一律放行
	cacheMock.EXPECT().
		Expire(gomock.Any(), sessionKey(token), gomock.Any()).
		AnyTimes().
		Return(nil)

	request.Header.Set(tokenHeaderKey, token)
}

// testUser 產生測試用 user 與其明文密碼，
// hashed_password 是真的 bcrypt 雜湊，login 測試才能驗證密碼比對邏輯。
func testUser(t *testing.T) (db.User, string) {
	password := "secret123"
	hashedPassword, err := util.HashPassword(password)
	require.NoError(t, err)

	return db.User{
		ID:             1,
		Username:       "alice",
		Email:          "alice@example.com",
		HashedPassword: hashedPassword,
		CreatedAt:      time.Now().UTC().Truncate(time.Second),
	}, password
}

func toAuthUser(user db.User) AuthUser {
	return AuthUser{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
	}
}

// parseResponse 解析統一回應格式 {code, msg, data}，回傳業務 code；
// data 給非 nil 的話會把 data 欄位解進去。比對 body 字串精確。
func parseResponse(t *testing.T, body *bytes.Buffer, data any) errcode.Code {
	var resp struct {
		Code errcode.Code    `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body.Bytes(), &resp))

	if data != nil && string(resp.Data) != "null" {
		require.NoError(t, json.Unmarshal(resp.Data, data))
	}
	return resp.Code
}
