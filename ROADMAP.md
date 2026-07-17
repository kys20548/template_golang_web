# Roadmap

功能演進紀錄與待辦。設計理由與實作細節見 [NOTES.md](NOTES.md)。

## 已完成

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
      `PUT /me/password` 改密碼（刪除目前 session 強制重登）+
      `admin_session:<uid>` 反查索引（一人一 session；刪帳號即時踢下線，見 NOTES「驗證層」）
- [x] **使用者軟刪除（前後台）** — `deleted_at` + partial unique index（同名可重新註冊、
      還原撞名回 409）、`includeDeleted` 列表參數、還原 API；後台帳號刪除即時踢 session、
      不能刪自己；前台刪除掛新權限 `user:write`。web 端有刪除（兩段式確認）／還原／含已刪除切換
- [x] **Render 部署 demo**（Postgres + Redis + Web Service + Static Site）
      — migration 隨容器啟動自動跑、CORS 收斂、production 模式；細節與免費方案的
      限制（Postgres 30 天到期、Pre-Deploy Command 鎖付費方案）見 NOTES「Render 部署」

## 待做（依序）

- [ ] **錢包加扣款** — 帳本表 `wallet_entries`（金額正負、備註、操作者、時間），
      加減與餘額檢查用單句 `UPDATE ... SET balance = balance + $1 ... AND balance + $1 >= 0`
      保證併發安全（不夠扣回 30001 餘額不足，錢包錯誤碼 3xxxx 段啟用）；
      新權限 `wallet:write`；web 加「加款/扣款」彈窗 + 異動明細列表頁；補併發測試
- [ ] **首頁 Dashboard 統計卡片** — 前台使用者數、錢包總餘額、今日操作數等，
      讓後台首頁不再只是導覽卡
- [ ] **Prometheus + Grafana 監控**（本地/demo 環境，Render 跑不了 sidecar）—
      gin middleware 收 request duration histogram（路由/狀態碼 label）、
      `/metrics` 不對外（獨立 port 或不過 CORS）、asynq exporter、
      業務指標（登入成功/失敗、錢包異動）；Prometheus + Grafana 進 docker compose，
      Grafana dashboard 用 provisioning 檔案進版控
- [ ] **kafka** — 有實際場景再加；原則：獨立 goroutine 執行、
      掛掉只記 log 不拖垮 HTTP server，graceful shutdown 時一併優雅關閉

## 明確不做

模板定位是 code，運維交給部署方：log 收集/alerting、secrets 管理、壓測。
