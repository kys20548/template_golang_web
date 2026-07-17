# template_golang_web

**Go 後台管理系統模板**——可以直接長出新後台專案的骨架：
驗證、RBAC 權限、操作稽核、排程任務、測試與部署配置都已就位，
附一套接好權限的 Vue 後台介面。

> 後端 **gin + viper + sqlc + PostgreSQL + Redis + asynq**，前端 **Vue 3 + Vite**。

## 功能總覽

**驗證與權限**

- Token 驗證層：Redis session + 反查索引（一人一 session，重複登入踢舊裝置）、
  bcrypt 密碼、登入失敗鎖定、sliding TTL 自動續期、改密碼強制重登
- 前台/後台 user 分離：登入者一律是後台 user（`admin_users`）；
  前台 user（`users`，公開註冊 + 錢包）只是管理對象
- RBAC：角色/權限四表 + `permMiddleware("user:read")` 路由層檢查，
  權限快照放 session、每個 request 零 DB 查詢
- 使用者軟刪除（前後台）＋還原：partial unique index 讓同名可重新註冊；
  刪除後台帳號即時踢下線、不能刪自己

**API 基礎建設**

- 統一回應格式 `{code, msg, data}`，業務狀態碼集中在 `errcode/` 管理
- 全域硬超時（真正取消進行中的 DB/Redis 操作）＋個別路由慢請求 log
- Request ID 貫穿鏈路（zerolog）、操作日誌（audit log）、統一 panic 回應
- Swagger 文件（development 環境 `/swagger/index.html`）、CORS、DB 連線池、graceful shutdown

**背景任務與測試**

- asynq 排程：scheduler 到點 enqueue、worker 執行，多 instance 以 `asynq.Unique` 去重
- mockgen + table-driven handler 測試，不需真實 DB/Redis；
  錢包併發扣款測試打真 DB 驗證（本地 DB 沒起就 skip）

**前端與部署**

- Vue 3 後台（`web/`）：登入、前台/後台使用者管理、錢包加扣款與異動明細、操作日誌；
  選單與按鈕依登入者權限顯隱，詳見 [web/README.md](web/README.md)
- Dockerfile（multi-stage 約 84MB，容器啟動自動跑 migration）＋
  GitHub Actions CI ＋ Render 部署 demo

## 專案結構

```
├── main.go              # 進入點：載入設定、連 DB/Redis、啟動 server、監聽關閉訊號
├── app.env              # viper 設定檔（環境變數可覆蓋）
├── api/                 # gin handler、路由、middleware、統一回應（*_test.go 為 handler 測試）
├── cache/               # Cache interface + Redis 實作（mock/ 由 mockgen 生成）
├── db/
│   ├── migration/       # golang-migrate 的 SQL migration
│   ├── query/           # sqlc 的 SQL query 定義（依 table 分檔）
│   ├── sqlc/            # sqlc 生成碼 + Store interface
│   └── mock/            # mockgen 生成的 Store mock
├── docs/                # swag 生成的 Swagger 文件
├── errcode/             # 業務狀態碼 enum 與對應訊息
├── scheduler/           # asynq 排程與背景任務
├── util/                # 設定載入等工具
├── web/                 # Vue 3 後台前端
└── entrypoint.sh        # Docker 容器進入點：先跑 migration 再啟動 main
```

## 快速開始

```bash
make postgres      # 啟動 PostgreSQL + Redis（docker compose）
make migrateup     # 執行 migration（需安裝 golang-migrate）
make server        # 啟動 API server（0.0.0.0:8080）

cd web && npm install && npm run dev   # 後台前端（http://localhost:5173）
```

後台種子帳號 `admin / admin123`（部署後請立即改密碼）。
API 一覽與試打用 Swagger：`http://localhost:8080/swagger/index.html`。

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

## 更多文件

- [NOTES.md](NOTES.md) — 設計理由與實作細節（驗證層、RBAC、軟刪除、超時控制、部署…）
- [ROADMAP.md](ROADMAP.md) — 功能演進紀錄與待辦
- [web/README.md](web/README.md) — 前端說明與 Render 部署
