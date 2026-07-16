#!/bin/sh
# Render Free tier 沒有 Pre-Deploy Command（付費方案才開放），改在容器啟動時
# 自己跑 migration；set -e 讓 migration 失敗時容器直接掛掉，不會用舊 schema 起服務。
set -e

migrate -path db/migration -database "$DB_SOURCE" -verbose up

exec /app/main
