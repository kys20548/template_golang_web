# template_golang_web

Golang Web 專案模板：**gin + viper + sqlc + PostgreSQL + Redis + asynq**。

## 功能特色

- **統一回應格式** `{code, msg, data}`，業務狀態碼集中在 `errcode/` 管理
- **Token 驗證層**：Redis session、bcrypt 密碼、登入失敗鎖定、即時登出、
  sliding TTL（活躍自動續期）、改密碼強制重新登入；前台/後台 user 分離，
  登入者一律是後台 user（`admin_users`）
- **RBAC 權限控制**：角色/權限四表 + `permMiddleware("user:read")` 路由層檢查，
  權限快照放 session、request 零 DB 查詢；後台可建帳號、指派角色
- **可觀測性**：Request ID 貫穿鏈路（zerolog）、操作日誌（audit log）、統一 panic 回應
- **API 超時控制**：全域硬超時真正取消 DB/Redis 操作 + 個別路由慢請求 log
- **排程與背景任務**：asynq scheduler 到點 enqueue、worker 執行，多 instance 以 `asynq.Unique` 去重
- **測試基礎設施**：mockgen + table-driven handler 測試，不需真實 DB/Redis
- **Swagger 文件**（development 環境 `/swagger/index.html`）、DB 連線池、CORS、graceful shutdown
- **Dockerfile（multi-stage，約 84MB）+ GitHub Actions CI**：image 內建 `migrate` CLI，
  容器啟動時（`entrypoint.sh`）自動跑 migration 才啟動服務，migration 失敗容器直接掛掉
- **Vue 後台前端**（`web/`）：登入、前台/後台使用者查詢、錢包列表、操作日誌、修改密碼，
  可獨立部署成 Render Static Site，詳見 [web/README.md](web/README.md)

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
├── util/                # 設定載入等工具
├── web/                 # Vue 3 後台前端，獨立部署見 web/README.md
└── entrypoint.sh        # Docker 容器進入點：先跑 migration 再啟動 main
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
curl -X POST http://localhost:8080/users -d '{"username":"danny","email":"danny@example.com","password":"secret123"}'  # 建立前台 user + 錢包
curl -X POST http://localhost:8080/login -d '{"username":"admin","password":"admin123"}'  # 後台 user 登入（migration 種子帳號，部署後請改密碼）

# 需驗證的路由（header 帶 token，登入者一律是後台 user；資源路由再各自檢查權限，無權限回 403 + 10007）
curl -H "token: <token>" http://localhost:8080/me                                   # 回登入者 + 權限快照
curl -X PUT -H "token: <token>" http://localhost:8080/me/password -d '{"old_password":"admin123","new_password":"newsecret456"}'  # 成功後需重新登入
curl -H "token: <token>" 'http://localhost:8080/wallets?pageNum=1&pageSize=10'      # 所有前台 user 的錢包（wallet:read）
curl -H "token: <token>" http://localhost:8080/users/1                              # 前台 user（user:read）
curl -H "token: <token>" 'http://localhost:8080/users?pageNum=1&pageSize=5'         # 前台 user 列表（user:read）
curl -H "token: <token>" 'http://localhost:8080/admin-users?pageNum=1&pageSize=10'  # 後台 user 列表含角色（admin_user:read）
curl -H "token: <token>" http://localhost:8080/roles                                # 角色與權限清單（admin_user:read）
curl -X POST -H "token: <token>" http://localhost:8080/admin-users -d '{"username":"viewer1","password":"viewer123","role_ids":[2]}'  # 建後台帳號（admin_user:write）
curl -X PUT -H "token: <token>" http://localhost:8080/admin-users/2/roles -d '{"role_ids":[1,2]}'  # 指派角色，整組取代（admin_user:write）
```

## Roadmap（尚未實作）

- [x] **後台管理頁面**（`web/`）：sidebar 版型 + 前台使用者列表/依 ID 查詢
      （`GET /users`、`GET /users/{id}`，分頁）、後台使用者列表（`GET /admin-users`，分頁）、
      前台使用者錢包列表（`GET /wallets`，分頁）、
      operation log 列表（`GET /operation-logs`，分頁）、改密碼（`PUT /me/password`）
- [x] **前台/後台 user 分離** — 本專案定位是後台系統：`admin_users` 表（登入、改密碼、
      session 都走它，migration 含種子帳號 admin/admin123）；`users` 表為前台 user
      （公開註冊 + 錢包），後台只做查詢
- [x] **RBAC 權限控制** — roles / permissions / role_permissions / admin_user_roles 四表 +
      `permMiddleware("user:read")` 權限中介層（code 用 resource:action，`*` 萬用）；
      權限快照登入時放進 Redis session，每個 request 零 DB 查詢（改角色需重新登入生效）。
      後台可建帳號、指派角色（整組取代）；角色/權限本身唯讀，異動用 migration/SQL 管。
      之後對齊既有 Java 系統的權限表結構時，只需要搬表和資料，middleware 判斷邏輯不動
- [x] **`/readyz` readiness 端點** — ping DB/Redis，給 LB / ASG(ELB health check) /
      k8s readiness probe 判斷「這台能不能收流量」；依賴掛了是摘流量等恢復，不是自殺重啟
- [x] **Session 補完** — sliding TTL（活躍使用者自動續期，不會用到一半被登出）+
      `PUT /me/password` 改密碼（刪除目前 session 強制重登；單一 session key 設計，
      不做反查索引，取捨見 NOTES「驗證層」）
- [x] **Render 部署 demo**（Postgres + Redis + Web Service + Static Site）
      — migration 隨容器啟動自動跑、CORS 收斂、production 模式；細節與免費方案的
      限制（Postgres 30 天到期、Pre-Deploy Command 鎖付費方案）見 NOTES「Render 部署」
- [ ] **kafka** — 有實際場景再加；原則：獨立 goroutine 執行、
      掛掉只記 log 不拖垮 HTTP server，graceful shutdown 時一併優雅關閉

明確不做（模板定位是 code，運維交給部署方）：log 收集/alerting、secrets 管理、
`/metrics`、壓測。

已完成項目的演進紀錄與設計細節見 [NOTES.md](NOTES.md)。
