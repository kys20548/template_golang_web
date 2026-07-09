# template_golang_web

Golang Web 專案模板：**gin + viper + sqlc + PostgreSQL + Redis**，含統一回應格式、token 驗證層、zerolog、graceful shutdown。

## 專案結構

```
├── main.go              # 進入點：載入設定、連 DB/Redis、啟動 server、監聽關閉訊號
├── app.env              # viper 設定檔（環境變數可覆蓋）
├── sqlc.yaml            # sqlc 設定（emit_interface: true）
├── api/                 # gin HTTP handler、路由、middleware、統一回應
├── cache/               # Cache interface + Redis 實作
├── errcode/             # 業務狀態碼 enum 與對應訊息
├── db/
│   ├── migration/       # golang-migrate 的 SQL migration
│   ├── query/           # sqlc 的 SQL query 定義
│   └── sqlc/            # sqlc 產生的程式碼 + Store interface
└── util/                # 設定載入等工具
```

## 統一回應格式

所有 API 回應都是 `{code, msg, data}`：

```json
{"code": 0, "msg": "success", "data": {...}}
{"code": 20001, "msg": "使用者不存在", "data": null}
```

`code` 定義在 `errcode/errcode.go`（0 成功、1xxxx 通用、2xxxx 使用者相關），
handler 用 `ok(ctx, data)` / `fail(ctx, httpStatus, code, err)` 回應，
err 只進 log 不回傳給 client。

## 驗證層

需要登入的路由掛 `authMiddleware`：header 帶 `token`，middleware 確認
token 存在 Redis（key: `session:<token>`）後，把 `AuthUser` 放進 gin context，
handler 用 `getAuthUser(ctx)` 取得登入者資訊。

```bash
TOKEN=$(curl -s -X POST localhost:8080/login -d '{"username":"danny"}' | jq -r .data.token)
curl -H "token: $TOKEN" localhost:8080/me
```

## Graceful Shutdown

main goroutine 阻塞等待 `SIGINT` / `SIGTERM`，收到訊號後呼叫
`http.Server.Shutdown(ctx)`：停止接收新連線，並在 `SHUTDOWN_TIMEOUT`
（預設 10s）內等待進行中的請求處理完成後才結束程式。

## 快速開始

```bash
make postgres      # 啟動 PostgreSQL + Redis（docker compose）
make migrateup     # 執行 migration（需安裝 golang-migrate）
make server        # 啟動 server（0.0.0.0:8080）
```

## 開發指令

| 指令 | 說明 |
|---|---|
| `make sqlc` | 重新產生 db/sqlc 程式碼 |
| `make migrateup` / `make migratedown` | 執行 / 回滾 migration |
| `make test` | 執行測試 |

## API 範例

```bash
# 公開路由
curl http://localhost:8080/healthz
curl -X POST http://localhost:8080/users -d '{"username":"danny","email":"danny@example.com"}'
curl -X POST http://localhost:8080/login -d '{"username":"danny"}'

# 需驗證的路由（header 帶 token）
curl -H "token: <token>" http://localhost:8080/me
curl -H "token: <token>" http://localhost:8080/users/1
curl -H "token: <token>" 'http://localhost:8080/users?pageNum=1&pageSize=5'
```
