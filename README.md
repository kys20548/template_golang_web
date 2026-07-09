# template_golang_web

Golang Web 專案模板：**gin + viper + sqlc + PostgreSQL**，含 graceful shutdown。

## 專案結構

```
├── main.go              # 進入點：載入設定、連 DB、啟動 server、監聽關閉訊號
├── app.env              # viper 設定檔（環境變數可覆蓋）
├── sqlc.yaml            # sqlc 設定（emit_interface: true）
├── api/                 # gin HTTP handler 與路由
├── db/
│   ├── migration/       # golang-migrate 的 SQL migration
│   ├── query/           # sqlc 的 SQL query 定義
│   └── sqlc/            # sqlc 產生的程式碼 + Store interface
└── util/                # 設定載入等工具
```

## Graceful Shutdown

main goroutine 阻塞等待 `SIGINT` / `SIGTERM`，收到訊號後呼叫
`http.Server.Shutdown(ctx)`：停止接收新連線，並在 `SHUTDOWN_TIMEOUT`
（預設 10s）內等待進行中的請求處理完成後才結束程式。

## 快速開始

```bash
make postgres      # 啟動 PostgreSQL（docker compose）
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
curl http://localhost:8080/healthz
curl -X POST http://localhost:8080/users -d '{"username":"danny","email":"danny@example.com"}'
curl http://localhost:8080/users/1
curl 'http://localhost:8080/users?page_id=1&page_size=5'
```
