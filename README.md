# template_golang_web

Golang Web 專案模板：**gin + viper + sqlc + PostgreSQL + Redis**，含統一回應格式、token 驗證層、zerolog、graceful shutdown。

## 專案結構

```
├── main.go              # 進入點：載入設定、連 DB/Redis、啟動 server、監聽關閉訊號
├── app.env              # viper 設定檔（環境變數可覆蓋）
├── sqlc.yaml            # sqlc 設定（emit_interface: true）
├── api/                 # gin HTTP handler、路由、middleware、統一回應
├── cache/               # Cache interface + Redis 實作
├── docs/                # swag 生成的 Swagger 文件（make swagger 重新生成）
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

## 操作日誌（audit log）

`auditLogMiddleware` 記錄所有寫入類操作（POST/PUT/PATCH/DELETE）到
`operation_logs` 表：誰（登入者，未登入為 null）、什麼時候、改了什麼
（method + path + request body）、結果 status code 與 request_id（可回查該次請求的完整 log）。

- body 中 key 含 `password` 的欄位會遮罩成 `***`，超過 4KB 截斷
- 寫入失敗只記 log，不影響已回應的請求
- `GET /operation-logs?pageNum=1&pageSize=10` 分頁查詢（需登入），最新的在前

## Swagger 文件

handler 註解生成 API 文件（對應 springdoc）。只在 development 環境提供：

```
http://localhost:8080/swagger/index.html
```

新增/修改 API 後執行 `make swagger` 重新生成 `docs/`。
需驗證的 API 在 Swagger UI 右上角 Authorize 填入登入取得的 token。

## API 超時控制

兩層設計：

- **硬超時**：全域 `API_TIMEOUT`（預設 10s），`timeoutMiddleware` 把 deadline
  掛在 request context 上，超時會真正取消進行中的 DB / Redis 操作，
  client 收到 `504 + {"code":10005,"msg":"請求處理逾時"}`。
  個別路由可再掛更短的 `timeoutMiddleware(2*time.Second)`，巢狀 context 誰短誰先到期。
- **慢請求 log**：個別路由掛 `slowLogMiddleware(2*time.Second)`，
  超過門檻只印 WARN `slow request`（含 request_id）方便排查，不中斷請求。

注意：deadline 能傳進 sqlc / go-redis 是靠 `router.ContextWithFallback = true`
（gin 預設 `ctx.Done()` 回空值），不要移除這行。
handler 的預設錯誤分支統一走 `failInternal(ctx, err)`，超時自動轉 504。

## 核心觀念筆記

### Middleware 順序（洋蔥模型）

middleware 的順序就是 `router.Use(...)` 裡寫的順序，結構是洋蔥：
`ctx.Next()` **之前**的程式碼在「進去」的路上跑，`ctx.Next()` **之後**的
程式碼在「出來」的路上跑，出來的順序與進去相反。

```
進去 →  requestID → httpLogger → timeout → cors → recovery → audit → handler
                                                                        │
出來 ←  requestID ← httpLogger ← timeout ← cors ← recovery ← audit ←────┘
```

| middleware | 進去時（Next 前） | 出來時（Next 後） |
|---|---|---|
| requestID | 產 id、掛 request-scoped logger | — |
| httpLogger | 記開始時間 | 印 access log（所以量得到 duration、status） |
| audit | 先讀走 request body | 寫 operation_logs（此時才有 status code 與登入者） |

順序是有講究的：httpLogger 要放外層才能把整條鏈的耗時都包住；
audit 要等 handler 做完才知道結果，所以工作放在 Next 之後。
新增 middleware 時要想：它的工作是進去做還是出來做、需要包住誰。

### Go context 觀念整理

有兩個完全不同的東西撞名叫 Context：

- **`context.Context`（Go 標準）**：4 個方法的小 interface（到期了沒、取消了沒、
  帶什麼值），是超時取消機制的本體。**不可變，沒有「設定」這個動作**，
  唯一的操作是「包一層生出新節點」，全 app 形成一棵樹。
- **`*gin.Context`（gin 的請求工具箱）**：gin 每個請求建一個的大結構體
  （Request、Writer、Params、`ctx.Set` 的置物格…）。`ctx.Next()` 是
  middleware 流程控制（換下一棒），跟標準 context 概念無關。

整棵樹長這樣，**工具箱不在樹上，是站在樹旁邊用 Request 指著某個節點**：

```
        context.Background()           ← 全 app 唯一的根（單例，到處呼叫拿到同一個）
         ├── pingCtx (5s)              ← main：啟動檢查 DB/Redis
         ├── shutdownCtx (10s)         ← listenSignal：優雅關閉
         └── 請求 A 的 context          ← net/http 幫每個請求長的（不是 gin）
               │
               +10s deadline           ←─────┐ timeoutMiddleware 長節點後把手指移過來
                  │        │                 │
           (+2s deadline)  WithoutCancel     │ 指著
                                             │
        ┌────────────────────────────────────┴──┐
        │ gin.Context（工具箱，站在樹外）           │
        │   Request ──其中的 Context 就是指標──────┘
        │   Writer / Params / Keys(置物格)
        └───────────────────────────────────────┘
```

規則整理：

- `WithTimeout(parent, d)` 長在**你給的 parent** 下面，不一定是第一層；
  巢狀 deadline 誰短誰先到期（個別路由 2s 疊在全域 10s 下就是這麼來的）
- 父節點取消/到期，子孫全部跟著取消；請求結束整條分支被回收，
  50 個並發請求就是 50 條互不相干的分支
- 取消是「合作式」：sqlc / go-redis 拿到 ctx 後邊做事邊監聽 `ctx.Done()`，
  到期就自己放棄，沒有人能從外部強制殺 goroutine
- `context.WithoutCancel(parent)`：值照樣繼承（request_id 還在），但父層的
  取消訊號傳不到它——audit 用它確保「請求超時被砍」這種最該稽核的紀錄寫得進去
- **`router.ContextWithFallback = true` 不能移除**：handler 把工具箱直接傳給
  sqlc 時，工具箱被問「到期了沒」預設裝死回「沒有時限」；開了 fallback
  它才會轉頭去問肚子裡的 `Request.Context()`，deadline 才真的生效。
  等價寫法是每個呼叫點改傳 `ctx.Request.Context()`，但容易漏寫

常見程式碼對照：

| 程式碼 | 屬於 | 做什麼 |
|---|---|---|
| `ctx.Next()` / `ctx.Set` / `ctx.Get` | gin 工具箱 | 流程控制 / 置物格 |
| `ctx.Request.Context()` | 標準 | 取出手指指著的樹上節點 |
| `context.Background()` | 標準 | 樹根（單例） |
| `context.WithTimeout` / `WithoutCancel` | 標準 | 在樹上包一層長新節點 |

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
| `make swagger` | 重新生成 Swagger 文件（需安裝 swag CLI） |
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
- [x] **操作日誌（audit log）** — `operation_logs` 表 + middleware 記錄寫入類操作
      （POST/PUT/DELETE）：誰、什麼時候、改了什麼
- [x] **統一 panic 回應** — `gin.Recovery()` panic 時回的是空 body 的 500，
      改用 `gin.CustomRecovery` 回統一的 `{code, msg, data}` 格式
- [x] **DB 連線池設定** — `sql.DB` 預設連線數無上限，`SetMaxOpenConns` /
      `SetMaxIdleConns` / `SetConnMaxLifetime` 從 config 讀
- [x] **Swagger 文件** — `swag` + `gin-swagger`，handler 註解生成 API 文件（對應 springdoc）

### 第三層：工程化

- [ ] **測試基礎設施** — `mockgen` 產 `db/mock`（`emit_interface` 就是為了這個）+
      handler 測試範例，參考 simple_bank 的模式
- [ ] **Dockerfile + CI** — multi-stage build 產出極小執行 image，GitHub Actions 跑 build/test
- [ ] **排程任務** — 對應 Java `@Scheduled`：簡單的用 `robfig/cron`，
      之後有 asynq 的話改用 asynq 內建 scheduler
- [ ] **asynq 背景任務 / kafka** — 有實際場景再加；原則：獨立 goroutine 執行、
      掛掉只記 log 不拖垮 HTTP server，graceful shutdown 時一併優雅關閉
