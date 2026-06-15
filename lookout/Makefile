SPECS := ../specs

.PHONY: generate gen
# Regenerate the ingest client + models from the shared OpenAPI spec so the
# bot↔app contract can never drift (REQUIREMENTS §14.1).
generate gen:
	go tool oapi-codegen -config generated/api/definitions-cfg.yaml $(SPECS)/definitions.yaml
	go tool oapi-codegen -config generated/api/cfg.yaml $(SPECS)/api.yaml

.PHONY: build
build: generate
	CGO_ENABLED=0 go build -o dist/lookout ./cmd/lookout

.PHONY: run
run: generate
	go run ./cmd/lookout

.PHONY: test
test:
	go test ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: fmt
fmt:
	gofmt -w internal cmd

.PHONY: tidy
tidy:
	go mod tidy
