DB_URL=postgresql://root:secret@localhost:5432/template_golang_web?sslmode=disable
GOBIN=$(shell go env GOPATH)/bin

postgres:
	docker compose up -d

createdb:
	docker exec -it template_golang_web_db createdb --username=root --owner=root template_golang_web

dropdb:
	docker exec -it template_golang_web_db dropdb template_golang_web

migrateup:
	migrate -path db/migration -database "$(DB_URL)" -verbose up

migratedown:
	migrate -path db/migration -database "$(DB_URL)" -verbose down

sqlc:
	sqlc generate

# 從 Store / Cache interface 產生 mock 實作，供 handler 測試用。
# 用法：mockgen -package <生成碼的 package 名> -destination <輸出檔> <interface 所在的 import path> <interface 名>
mock:
	$(GOBIN)/mockgen -package mockdb -destination db/mock/store.go github.com/kys20548/template_golang_web/db/sqlc Store
	$(GOBIN)/mockgen -package mockcache -destination cache/mock/cache.go github.com/kys20548/template_golang_web/cache Cache

swagger:
	$(GOBIN)/swag init --parseDependency --parseInternal

server:
	go run main.go

test:
	go test -v -cover ./...

.PHONY: postgres createdb dropdb migrateup migratedown sqlc mock swagger server test
