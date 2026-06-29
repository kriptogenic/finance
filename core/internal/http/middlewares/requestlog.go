package middlewares

import (
	"context"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"finance/generated/api"
)

// IngestRequestLogger is an oapi-codegen strict middleware that logs requests
// to the ingest operations, capturing method, path, latency and any error.
// Non-ingest operations pass through untouched.
func IngestRequestLogger(logger *zap.Logger) api.StrictMiddlewareFunc {
	return func(f api.StrictHandlerFunc, operationID string) api.StrictHandlerFunc {
		if !strings.HasPrefix(operationID, "Ingest") {
			return f
		}

		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			start := time.Now()
			resp, err := f(ctx, w, r, request)

			fields := []zap.Field{
				zap.String("operation", operationID),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Duration("duration", time.Since(start)),
				zap.String("remote_ip", r.RemoteAddr),
			}
			if err != nil {
				logger.Error("ingest request failed", append(fields, zap.Error(err))...)
			} else {
				logger.Info("ingest request", fields...)
			}

			return resp, err
		}
	}
}
