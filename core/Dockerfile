# Build context is the REPO ROOT (the shared ../specs dir lives outside core/).
# Generated Go (generated/api/*.go) is gitignored, so we regenerate it from the
# OpenAPI spec at build time — same step as `make generate`.
FROM golang:1.26-bookworm AS build

WORKDIR /app

# Shared OpenAPI spec, needed for codegen (core/Makefile: SPECS := ../specs).
COPY specs/ ./specs/

# Module graph first for a cacheable dependency layer.
COPY core/go.mod core/go.sum ./core/
WORKDIR /app/core
RUN go mod download

# Source, then regenerate the spec-derived code and build static binaries.
COPY core/ ./
RUN go tool oapi-codegen -config generated/api/definitions-cfg.yaml ../specs/definitions.yaml \
 && go tool oapi-codegen -config generated/api/cfg.yaml ../specs/api.yaml
RUN CGO_ENABLED=0 go build -trimpath -o /out/server  ./cmd/server \
 && CGO_ENABLED=0 go build -trimpath -o /out/migrate ./cmd/migrate

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 app
WORKDIR /app

COPY --from=build /out/server /out/migrate /usr/local/bin/
# Migrations are read from the filesystem (golang-migrate file:// source).
COPY core/migrations/ ./migrations/
COPY core/entrypoint.sh /usr/local/bin/entrypoint.sh

ENV DB_MIGRATIONS_PATH=migrations
USER app
EXPOSE 8080

# Apply pending migrations, then start the API server. Invoked via sh so the
# script needs no executable bit (kept 0644 to avoid build-context mode quirks).
ENTRYPOINT ["sh", "/usr/local/bin/entrypoint.sh"]
