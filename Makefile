.PHONY: run build test migrate db-create db-drop db-reset

PORT ?= 8080
DB_HOST ?= 127.0.0.1
DB_USER ?= postgres
DB_PASSWORD ?= postgres
DB_NAME ?= duitno
DB_PORT ?= 5432

run:
	cd cmd/api && go run main.go

build:
	cd cmd/api && go build -o bin/duitno main.go

test:
	go test ./...

migrate:
	cd cmd/api && go run main.go

db-drop:
	PGPASSWORD=$(DB_PASSWORD) psql -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) -d postgres -c "DROP DATABASE IF EXISTS $(DB_NAME);"

db-reset: db-drop run
