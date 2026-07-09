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

swagger:
	$(GOBIN)/swag init --parseDependency --parseInternal

server:
	go run main.go

test:
	go test -v -cover ./...

.PHONY: postgres createdb dropdb migrateup migratedown sqlc swagger server test
