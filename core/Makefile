.PHONY: build
build:
	go build -v -o dist/finance ./cmd/server/

.PHONY: run
run:
	go run ./cmd/server/

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
