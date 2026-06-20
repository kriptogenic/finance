SPECS := ../specs

.PHONY: generate gen
generate gen:
	go tool oapi-codegen -config generated/api/definitions-cfg.yaml $(SPECS)/definitions.yaml
	go tool oapi-codegen -config generated/api/cfg.yaml $(SPECS)/api.yaml

.PHONY: build
build:
	make generate
	go build -v -o dist/finance ./cmd/server/

.PHONY: run
run:
	make generate
	go run ./cmd/server/

.PHONY: migrate
migrate:
	go run ./cmd/migrate up

.PHONY: migrate-fresh
migrate-fresh:
	go run ./cmd/migrate fresh

.PHONY: vapid-keys
vapid-keys:
	go run ./cmd/vapidkeys

.PHONY: seed
seed:
	go run ./cmd/seed

.PHONY: seed-reset
seed-reset:
	go run ./cmd/seed reset

.PHONY: test
test:
	go test -race -timeout 60s ./... -count=1

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: kill
kill:
	pkill -f "go run ./cmd/server/" 2>/dev/null; pkill -f "cmd/server" 2>/dev/null

.PHONY: lint
lint:
	golangci-lint run

.PHONY: style
style:
	golangci-lint fmt

.DEFAULT_GOAL := build
