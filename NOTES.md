# 設計筆記

各功能的設計理由與實作細節。快速上手與指令見 [README](README.md)。

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

**Session 生命週期**（單一 Redis key：`session:<token>` → AuthUser JSON，
TTL = `TOKEN_DURATION`）：

- **sliding TTL**：authMiddleware 驗證通過就把 TTL 重設回 `TOKEN_DURATION`——
  活躍使用者不會用到一半被登出，閒置滿 `TOKEN_DURATION` 才過期。
  續期失敗不影響本次請求（session 還沒過期），只記 WARN。
- **改密碼**：`PUT /me/password` 驗舊密碼 → 改密碼 → 刪除目前的 session，
  強制重新登入。
- **取捨（刻意保持簡單）**：沒有 user → tokens 的反查索引，所以同一帳號的
  其他 session（正常情況不會有——不會登入了一次又登入第二次）不會被踢，
  留到 TTL 自然過期。之後若真需要「改密碼/停用帳號踢掉全部裝置」，
  再加 `user_sessions:<id>` set 反查索引。

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

## 排程任務與背景任務（asynq）

`scheduler/` 用 asynq（Redis-backed）實作，每個 instance 內同時跑兩個角色：

- **Scheduler**：cron 到點把任務 enqueue 進 Redis
  （`OPERATION_LOG_CLEANUP_CRON`，預設 `0 0 * * *` 每天凌晨，以本地時區解讀）
- **Worker**：從 Redis 取出任務執行，handler 回傳 error 時 asynq 自動重試

每個任務在自己的 `task_xxx.go` 裡定義一個 `periodicTask`（cron、enqueue 選項、handler），
並在 `periodicTasks()` 加一行；`Start()` 只負責迴圈註冊。
enqueue 選項是**每個任務自己的決定**——要不要去重、TTL 多長，都寫在任務自己的定義裡。

**多 instance 去重**（可選）：每台的 scheduler 都會在同一時間 enqueue，
任務掛 `asynq.Unique(ttl)` 的話，會以 (queue, type, payload) 在 Redis 上鎖——
搶輸的那台拿到 `ErrDuplicateTask`（預期行為，記 info 跳過），任務只會被一個 worker 執行一次。
TTL 大於各 instance 時鐘誤差即可（清理任務用 1 分鐘）。
enqueue 結果統一在 `PostEnqueueFunc` 記 log。

目前任務：`operation_log:cleanup` — 刪除超過保留期限
（`OPERATION_LOG_RETENTION_MONTHS`，預設 3 個月）的 operation_logs。
就算當天重試全失敗，隔天排程也會把積欠的資料一起清掉，不需額外補償。

失敗哲學：啟動期 fail-fast——scheduler 起不來就 `log.Fatal`，不啟動 HTTP server
（啟動需要的東西一定要齊全，不允許服務帶著缺功能上線）；runtime 的任務執行失敗
則只記 log 並交給 asynq 重試，不影響 HTTP server。
graceful shutdown 先停 scheduler（不再產生新任務）再停 worker（等進行中任務跑完）。

本機實測技巧：用環境變數把 cron 覆蓋成每分鐘一次，起兩個 instance 看去重：

```bash
HTTP_SERVER_ADDRESS=0.0.0.0:8081 OPERATION_LOG_CLEANUP_CRON="* * * * *" go run main.go
HTTP_SERVER_ADDRESS=0.0.0.0:8082 OPERATION_LOG_CLEANUP_CRON="* * * * *" go run main.go
```

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

## 健康檢查（liveness / readiness）

兩個端點，語意不同：

- **`/healthz`（liveness）**：進程活著就回 200，不看依賴。給「要不要重啟這個進程」用。
- **`/readyz`（readiness）**：ping DB 與 Redis，都通才回 200，否則 `503 + 10006 服務未就緒`。
  給 LB / ASG（health check type 設 ELB，target group 指到這裡）/ k8s readiness probe
  判斷「這台能不能收流量」。

為什麼依賴掛了是回 503 而不是讓進程自己退出：DB 抖動 30 秒，全部 instance
同時自殺 → ASG 慢慢補機器 → 新機器起來 DB 還沒好又死，一次抖動放大成全面停機。
正確行為是 instance 活著但宣告未就緒，LB 摘掉流量，依賴恢復後自動回歸（實測過：
`docker pause` Redis 後 /readyz 回 503，unpause 後不用重啟就恢復 200）。

/readyz 掛了更短的 `timeoutMiddleware(2s)`：探針要快進快出，DB 連不上時
2 秒內就回 503，不佔著探針等全域的 10s。

注意與啟動檢查的分工：啟動期依賴連不上是 fail-fast（`log.Fatal` 不起服務），
runtime 依賴抖動是 readiness 摘流量等恢復——一個是「不要帶病上線」，
一個是「上線後生病不要自殺」。

## Graceful Shutdown

main goroutine 阻塞等待 `SIGINT` / `SIGTERM`，收到訊號後呼叫
`http.Server.Shutdown(ctx)`：停止接收新連線，並在 `SHUTDOWN_TIMEOUT`
（預設 10s）內等待進行中的請求處理完成，之後再依序關閉 scheduler 與 worker。

## Docker 與 CI

**Dockerfile**（multi-stage）：builder 層用完整 Go 工具鏈編譯靜態執行檔，
最終 image 只有 alpine + 執行檔 + app.env（約 70MB）。兩個細節：

- 有裝 `tzdata`，但 **image 不自己設 TZ**——時區以部署環境注入的 `TZ` 環境變數為準
  （一般原則：跟部署機器走，程式不要自作主張）。沒注入 TZ 時容器跑 UTC，
  scheduler 的 cron「凌晨」就是 UTC 半夜，部署時要注意
- image 內的 `app.env` 只是讓 viper 能啟動的底，實際部署用環境變數覆蓋

migration 不在 image 裡執行，部署時另外跑 `migrate` CLI（或 CI/CD pipeline 的獨立步驟）。

**CI**（`.github/workflows/ci.yml`）：push / PR 觸發，跑 `go build` → `go vet` →
`go test`（全 mock，不需 DB/Redis service），過了再驗證 docker image 蓋得起來。

## 測試

handler 測試不需要真實 DB / Redis：`Store` 與 `Cache` 都是 interface（`emit_interface: true`
就是為了這個），用 `mockgen` 產生 mock（`db/mock`、`cache/mock`，interface 變動後 `make mock`
重新生成）。測試用 `httptest` 對完整 middleware 鏈 + handler 發請求——進入點跟真實流量
一樣是 `ServeHTTP`——驗證 HTTP status、業務 code 與回應內容。參考 simple_bank 的模式。

寫 handler 測試的要點（範例見 `api/*_test.go`）：

- **table-driven**：同一支 API 的所有情境列成一張表，每個 case 三件事——
  `buildStubs`（排 mock 劇本）、`setupAuth`（怎麼登入）、`checkResponse`（驗什麼）。
  gomock 的 `Times(n)` 同時驗證互動：該呼叫的有呼叫、不該呼叫的沒呼叫
  （例如參數驗證失敗就不該打 DB、登入鎖定就不查帳號）。
- **共用 helper 集中在 `main_test.go`**：`newTestServer`（組 server，config 直接給值不讀
  app.env）、`setupAuth`（header 放 token + mock session）、`parseResponse`（解統一回應）。
  基礎設施變動（NewServer 簽名、登入機制）只需改 helper，測試本體不用動。
- **寫入類請求（POST/PUT/DELETE）**：會經過 auditLogMiddleware，mock store 要多排一筆
  `CreateOperationLog` 劇本，否則 gomock 報 unexpected call。
- **bcrypt 參數比對**：同一密碼每次雜湊結果不同，不能用 `gomock.Eq` 整包比對，
  用自訂 matcher（`eqCreateUserTxParams`）以 `CheckPassword` 驗證雜湊來源後再比其餘欄位。
- **middleware 單獨測**：`authMiddleware` 用裸 gin router 只掛它一個來測（單元視角），
  與走完整鏈的 handler 測試（整合視角）互補。
- 有些 case 驗的是**安全性質**：帳號不存在與密碼錯誤回同一錯誤碼、回應不含
  `hashed_password`、錢包只能用 session 裡的 user id 查。
