# syntax=docker/dockerfile:1
FROM golang:1.26-bookworm AS build

WORKDIR /app

COPY core/go.mod core/go.sum ./core/
COPY lookout/go.mod lookout/go.sum ./lookout/
WORKDIR /app/lookout
RUN go mod download

COPY specs/ /app/specs/
COPY core/ /app/core/
COPY lookout/ ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    go tool oapi-codegen -config generated/core/definitions-cfg.yaml ../specs/definitions.yaml \
 && go tool oapi-codegen -config generated/core/cfg.yaml ../specs/api.yaml
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -o /out/lookout ./cmd/lookout

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 app

COPY --from=build /out/lookout /usr/local/bin/lookout

RUN mkdir -p /data && chown app:app /data
WORKDIR /data

ENV SESSION_FILE=session.json \
    STATE_FILE=lookout-state.json
USER app

ENTRYPOINT ["lookout"]
