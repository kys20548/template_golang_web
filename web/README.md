# 後台前端

Vue 3 + Vite + Vue Router，純 JS（不含 TypeScript / Pinia / UI 框架，規模用不到，
之後頁面變多再評估要不要加）。登入後為 sidebar + 內容區的後台版型，涵蓋前台/後台
使用者查詢、前台使用者錢包列表、操作日誌、修改密碼；登入帳號是後台 user（`admin_users`）。

對接的後端 API 見專案根目錄 [README](../README.md) 與 [NOTES.md](../NOTES.md)。

## 開發

```bash
npm install
npm run dev      # http://localhost:5173，打 .env.development 指定的 API（預設 localhost:8081）
npm run build    # 輸出到 dist/
```

## 目錄

```
src/
  api/client.js     # fetch 包裝：帶 token header、解 {code,msg,data} envelope
  auth/session.js   # token/user 存取（localStorage）
  router/index.js   # /login + AppShell 底下的巢狀路由，未登入導向 /login 的守衛
  components/
    AppShell.vue      # 登入後的版型：sidebar 導覽 + topbar（使用者 + 登出）
    Pagination.vue    # 分頁列表共用的上一頁/下一頁元件
  views/
    LoginView.vue          # 登入（後台 user）
    DashboardView.vue      # 首頁：快捷入口卡片
    UsersView.vue          # 前台使用者分頁列表 + 依 ID 查詢
    AdminUsersView.vue     # 後台使用者分頁列表
    WalletsView.vue        # 前台使用者錢包分頁列表
    OperationLogsView.vue  # 操作日誌分頁列表
    ChangePasswordView.vue # 修改密碼（成功後導回登入頁）
```

## 部署（Render Static Site）

Root Directory `web`、Build Command `npm install && npm run build`、
Publish Directory `dist`，環境變數 `VITE_API_BASE_URL` 指向後端 Web Service 網址。
SPA 路由要在 Render 的 **Redirects/Rewrites** 頁面另外加規則
（`/* → /index.html`，Action 選 **Rewrite**，不是 Redirect）——
Render 不支援 Netlify 風格的 `_redirects` 檔案。細節見根目錄 NOTES.md「Render 部署」。
