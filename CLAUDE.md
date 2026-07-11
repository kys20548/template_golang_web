# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 溝通與工作方式

- 以繁體中文（zh-TW）回覆。
- **只有在使用者明確說「commit」時才 commit**，不要主動 commit。
- 功能做完要端到端實測（起 server 用 curl 打），不要只靠編譯通過。測試時用環境變數覆蓋端口避免撞到開發中的實例：`HTTP_SERVER_ADDRESS=0.0.0.0:8081 go run main.go`（viper AutomaticEnv 會覆蓋 app.env）。

## 常用指令

```bash
make postgres      # 啟動 PostgreSQL + Redis（docker compose）
make migrateup     # 執行 migration（migrate CLI 在 /usr/local/bin/migrate）
make sqlc          # 重新生成 db/sqlc/（sqlc 用 brew 裝的，go install 在 macOS 會編譯失敗）
make mock          # 重新生成 db/mock/ 與 cache/mock/（mockgen 在 $(go env GOPATH)/bin）
make swagger       # 重新生成 docs/（swag 在 $(go env GOPATH)/bin）
make server        # 啟動 server（0.0.0.0:8080）
make test          # go test -v -cover ./...
go build ./... && go vet ./...   # 改完碼的基本檢查
```

DB 直接查詢：`docker exec template_golang_web_db psql -U root -d template_golang_web -c "..."`

## 生成碼（絕對不要手改）

- `db/sqlc/` — 改 `db/query/*.sql` 後 `make sqlc`
- `db/mock/`、`cache/mock/` — Store / Cache interface 變動後 `make mock`
- `docs/` — 改 handler 的 Swagger 註解後 `make swagger`

有 PreToolUse hook 會擋這兩個目錄的編輯。

## 架構

flat layout（仿 simple_bank）：`main.go` 在根目錄做依賴注入，`api/`（handler、middleware、路由）、`db/`（migration、query、sqlc 生成碼 + Store interface）、`cache/`（Cache interface + Redis 實作）、`errcode/`（業務狀態碼）、`util/`（config、password）。

**啟動與關閉**：main 先 Ping DB 和 Redis（失敗就 `log.Fatal`，不啟動 HTTP server——`sql.Open` 是惰性的所以必須顯式 Ping）；gin engine 掛在 `http.Server` 上，SIGINT/SIGTERM 觸發 `server.Shutdown`（timeout 由 `SHUTDOWN_TIMEOUT` 控制）。

**失敗哲學**：runtime 元件各自獨立失敗——背景元件掛掉只記 log，絕不拖垮 HTTP server（不用 errgroup 做生命週期耦合）；啟動期則是 fail-fast。

**Middleware 鏈（洋蔥，順序有講究）**：`requestID → httpLogger → timeout → cors → recovery → audit`。詳細原理見 README「核心觀念筆記」。

## 專案慣例（新增 API 時必守）

1. **統一回應** `{code, msg, data}`：成功用 `ok(ctx, data)`，錯誤用 `fail(ctx, httpStatus, code, err)`；err 只進 log（經 ctx.Error → httpLogger），永不回給 client。handler 的預設錯誤分支一律走 `failInternal(ctx, err)`（自動把 `context.DeadlineExceeded` 轉成 504 + 10005）。
2. **errcode 編碼**：0 成功、1xxxx 通用、2xxxx 使用者、3xxxx 錢包，新業務模組依序 4xxxx…；在 `errcode/errcode.go` 加常數 + messages 對應。
3. **分頁列表**回 `PageResult{pageNum, pageSize, total, list}`，query 參數 binding `pageNum`/`pageSize`（min=5,max=50）。
4. **驗證**：需登入的路由掛在 `authRoutes` group（header `token` → Redis `session:<token>`）；handler 用 `getAuthUser(ctx)` 取登入者。**查「自己的」資源一律從 context 拿 user，不接受 request 參數指定 user**。context 只放 handler 拿不到的東西（token 在 header 自己拿）。
5. **log**：handler 內一律用 `getLogger(ctx)`（帶 request_id 的 request-scoped logger），不要用全域 `log`。
6. **敏感欄位**：user 對外回應一律走 `userResponse`，不得洩漏 `hashed_password`；新的對外結構同理。
7. **每支 handler 要有 Swagger 註解**（@Summary/@Tags/@Success/@Router，需登入的加 @Security TokenAuth），改完 `make swagger`。
8. `server.go` 的 `router.ContextWithFallback = true` **不能移除**，否則 API 超時的 deadline 傳不進 sqlc/go-redis（原理見 README）。
9. migration 檔名 `00000N_name.up.sql` / `.down.sql` 成對遞增，改完 `make migrateup` 驗證。
10. **新增 API 要補 handler 測試**：table-driven，用 `db/mock` + `cache/mock`，共用 helper 在 `api/main_test.go`（`newTestServer` / `setupAuth` / `parseResponse`）；POST/PUT/DELETE 要多排一筆 `CreateOperationLog` 劇本（audit middleware 會呼叫）；含 bcrypt 雜湊的參數用自訂 matcher 比對（見 `eqCreateUserTxParams`）。要點詳見 README「測試」。

## 新增一支 API 的流程

`db/query/*.sql` 寫 SQL → `make sqlc` → handler（統一回應 + Swagger 註解 + failInternal）→ `server.go` 掛路由（決定要不要 auth / slowLog / 更短 timeout）→ `make swagger` → handler 測試（慣例 10）→ `go build ./... && go vet ./... && make test` → 起 server curl 實測 → 更新 README。

## Roadmap

未實作項目見 README「Roadmap（尚未實作）」：RBAC（開工前先對齊使用者既有 Java 系統的權限表結構）、Dockerfile + CI、排程、asynq/kafka。
