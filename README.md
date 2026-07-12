# template_golang_web

Golang Web 專案模板：**gin + viper + sqlc + PostgreSQL + Redis + asynq**。

## 功能特色

- **統一回應格式** `{code, msg, data}`，業務狀態碼集中在 `errcode/` 管理
- **Token 驗證層**：Redis session、bcrypt 密碼、登入失敗鎖定、即時登出
- **可觀測性**：Request ID 貫穿鏈路（zerolog）、操作日誌（audit log）、統一 panic 回應
- **API 超時控制**：全域硬超時真正取消 DB/Redis 操作 + 個別路由慢請求 log
- **排程與背景任務**：asynq scheduler 到點 enqueue、worker 執行，多 instance 以 `asynq.Unique` 去重
- **測試基礎設施**：mockgen + table-driven handler 測試，不需真實 DB/Redis
- **Swagger 文件**（development 環境 `/swagger/index.html`）、DB 連線池、CORS、graceful shutdown
- **Dockerfile（multi-stage，約 70MB）+ GitHub Actions CI**

設計理由與實作細節見 **[NOTES.md 設計筆記](NOTES.md)**。

## 專案結構

```
├── main.go              # 進入點：載入設定、連 DB/Redis、啟動 server、監聽關閉訊號
├── app.env              # viper 設定檔（環境變數可覆蓋）
├── sqlc.yaml            # sqlc 設定（emit_interface: true）
├── api/                 # gin HTTP handler、路由、middleware、統一回應（*_test.go 為 handler 測試）
├── cache/               # Cache interface + Redis 實作
│   └── mock/            # mockgen 產生的 Cache mock（make mock 重新生成）
├── docs/                # swag 生成的 Swagger 文件（make swagger 重新生成）
├── errcode/             # 業務狀態碼 enum 與對應訊息
├── db/
│   ├── migration/       # golang-migrate 的 SQL migration
│   ├── query/           # sqlc 的 SQL query 定義
│   ├── sqlc/            # sqlc 產生的程式碼 + Store interface
│   └── mock/            # mockgen 產生的 Store mock（make mock 重新生成）
├── scheduler/           # asynq 排程與背景任務（cron enqueue + worker 執行）
└── util/                # 設定載入等工具
```

## 快速開始

```bash
make postgres      # 啟動 PostgreSQL + Redis（docker compose）
make migrateup     # 執行 migration（需安裝 golang-migrate）
make server        # 啟動 server（0.0.0.0:8080）
```

或用 Docker：

```bash
docker build -t template_golang_web .
docker run -p 8080:8080 \
  -e DB_SOURCE="postgresql://root:secret@host.docker.internal:5432/template_golang_web?sslmode=disable" \
  -e REDIS_ADDRESS="host.docker.internal:6379" \
  template_golang_web
```

## 開發指令

| 指令 | 說明 |
|---|---|
| `make sqlc` | 重新產生 db/sqlc 程式碼 |
| `make mock` | 重新產生 db/mock 與 cache/mock（需安裝 mockgen） |
| `make swagger` | 重新生成 Swagger 文件（需安裝 swag CLI） |
| `make migrateup` / `make migratedown` | 執行 / 回滾 migration |
| `make test` | 執行測試（不需要 DB / Redis） |

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

- [ ] **RBAC 權限控制** — roles / permissions / user_roles 表 + `permMiddleware("user:delete")`
      權限中介層；權限清單登入時放進 Redis session。開工前先對齊既有 Java 系統的權限表結構
- [ ] **kafka** — 有實際場景再加；原則：獨立 goroutine 執行、
      掛掉只記 log 不拖垮 HTTP server，graceful shutdown 時一併優雅關閉

已完成項目的演進紀錄與設計細節見 [NOTES.md](NOTES.md)。
