# syntax=docker/dockerfile:1
FROM golang:1.26-bookworm AS build

WORKDIR /app

COPY core/go.mod core/go.sum ./core/
WORKDIR /app/core
RUN go mod download

COPY specs/ /app/specs/
COPY core/ ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    go tool oapi-codegen -config generated/api/definitions-cfg.yaml ../specs/definitions.yaml \
 && go tool oapi-codegen -config generated/api/cfg.yaml ../specs/api.yaml
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -o /out/server  ./cmd/server \
 && CGO_ENABLED=0 go build -trimpath -o /out/migrate ./cmd/migrate

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 app
WORKDIR /app

COPY --from=build /out/server /out/migrate /usr/local/bin/
COPY core/migrations/ ./migrations/

ENV DB_MIGRATIONS_PATH=migrations
USER app
EXPOSE 8080

ENTRYPOINT ["sh", "-c", "migrate up && exec server"]
