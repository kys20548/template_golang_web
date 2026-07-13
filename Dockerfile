# Build stage：完整 Go 工具鏈只存在於這一層，不會進最終 image
FROM golang:1.26-alpine AS builder
WORKDIR /app

# 先只複製 go.mod/go.sum 下載依賴，程式碼沒動時這層有 cache，重建很快
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0 產出靜態執行檔（alpine 沒有 glibc）；-s -w 去掉符號表縮小體積
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o main .

# Run stage：只放執行檔 + 設定檔
FROM alpine:3.22
WORKDIR /app

# tzdata：讓部署環境注入的 TZ 環境變數生效（沒有 tzdata 的話 TZ 設了也沒用）。
# image 不自己設 TZ——時區是部署環境的決定；沒注入 TZ 時容器跑 UTC，
# scheduler 的 cron「凌晨」就是 UTC 半夜，部署時要自己想清楚
# ca-certificates：之後打外部 HTTPS API 會用到
RUN apk add --no-cache tzdata ca-certificates

COPY --from=builder /app/main .
# viper 要求 app.env 存在才能啟動；實際部署用環境變數覆蓋其中的值
COPY app.env .

EXPOSE 8080
ENTRYPOINT ["/app/main"]
