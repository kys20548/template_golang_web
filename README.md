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

登入安全：密碼以 bcrypt 儲存；帳號不存在與密碼錯誤回同一個錯誤碼（20003）；
連續失敗 5 次鎖定 15 分鐘（Redis 計數器）；`POST /logout` 刪除 session 即時登出。
user 回應一律走 `userResponse`，不會帶出 hashed_password。

```bash
TOKEN=$(curl -s -X POST localhost:8080/login -d '{"username":"danny","password":"secret123"}' | jq -r .data.token)
curl -H "token: $TOKEN" localhost:8080/me
```

## Request ID 貫穿鏈路

`requestIDMiddleware` 為每個請求產生 UUID（client 有帶 `X-Request-Id` 就沿用，
方便跨服務串接），放進 response header 回給 client，並把帶有 `request_id` 的
logger 放進 request context。handler 內記 log 一律用 `getLogger(ctx)`，
同一請求的所有 log（含 access log）都會帶同一個 `request_id`，
可以直接串起來查（對應 Java 的 MDC + traceId）。

## 統一 panic 回應

`gin.CustomRecoveryWithWriter` + `recoveryHandler`：handler panic 時
client 一律收到統一的 `{"code":10001,"msg":"系統內部錯誤","data":null}`，不會外洩細節。
server 端的 stack trace 依環境輸出：development 用 gin 內建的多行可讀格式，
production 由 zerolog 記成單行 JSON（含 request_id）給 log 收集器。

## DB 連線池

`sql.DB` 預設開啟連線數無上限，尖峰時會把 DB 塞爆。`app.env` 提供：
`DB_MAX_OPEN_CONNS`（最大連線數）、`DB_MAX_IDLE_CONNS`（閒置保留數）、
`DB_CONN_MAX_LIFETIME`（連線最長存活時間，定期換新避免被 DB 或 LB 靜默斷線）。

## CORS

`app.env` 的 `CORS_ALLOW_ORIGINS` 控制允許的跨域來源：`*` 允許全部（開發用），
production 改成逗號分隔清單，例如 `https://admin.example.com,https://app.example.com`。

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
curl -X POST http://localhost:8080/users -d '{"username":"danny","email":"danny@example.com","password":"secret123"}'
curl -X POST http://localhost:8080/login -d '{"username":"danny","password":"secret123"}'

# 需驗證的路由（header 帶 token）
curl -H "token: <token>" http://localhost:8080/me
curl -H "token: <token>" http://localhost:8080/wallet   # 查登入者自己的錢包（user 來自 context）
curl -H "token: <token>" http://localhost:8080/users/1
curl -H "token: <token>" 'http://localhost:8080/users?pageNum=1&pageSize=5'
```

## Roadmap（尚未實作）

### 第一層：核心功能

- [ ] **RBAC 權限控制** — roles / permissions / user_roles 表 + `permMiddleware("user:delete")`
      權限中介層；權限清單登入時放進 Redis session。開工前先對齊既有 Java 系統的權限表結構

### 第二層：上線運維

- [x] **Request ID 貫穿鏈路** — middleware 產 UUID 放進 context 與 response header，
      zerolog 以 `log.With().Str("request_id", ...)` 帶著走，同一請求的 log 可以串起來（對應 Java MDC + traceId）
- [ ] **操作日誌（audit log）** — `operation_logs` 表 + middleware 記錄寫入類操作
      （POST/PUT/DELETE）：誰、什麼時候、改了什麼
- [x] **統一 panic 回應** — `gin.Recovery()` panic 時回的是空 body 的 500，
      改用 `gin.CustomRecovery` 回統一的 `{code, msg, data}` 格式
- [x] **DB 連線池設定** — `sql.DB` 預設連線數無上限，`SetMaxOpenConns` /
      `SetMaxIdleConns` / `SetConnMaxLifetime` 從 config 讀
- [ ] **Swagger 文件** — `swag` + `gin-swagger`，handler 註解生成 API 文件（對應 springdoc）

### 第三層：工程化

- [ ] **測試基礎設施** — `mockgen` 產 `db/mock`（`emit_interface` 就是為了這個）+
      handler 測試範例，參考 simple_bank 的模式
- [ ] **Dockerfile + CI** — multi-stage build 產出極小執行 image，GitHub Actions 跑 build/test
- [ ] **排程任務** — 對應 Java `@Scheduled`：簡單的用 `robfig/cron`，
      之後有 asynq 的話改用 asynq 內建 scheduler
- [ ] **asynq 背景任務 / kafka** — 有實際場景再加；原則：獨立 goroutine 執行、
      掛掉只記 log 不拖垮 HTTP server，graceful shutdown 時一併優雅關閉
