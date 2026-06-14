package middlewares

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	middleware "github.com/oapi-codegen/nethttp-middleware"

	"finance/generated/api"
)

const bearerPrefix = "Bearer "

// OpenAPIRequestValidator validates incoming requests against the OpenAPI spec
// (path, query, body schemas) and authenticates operations that declare the
// IngestAuth bearer scheme against ingestToken. An empty token disables auth
// (local-only deployments).
func OpenAPIRequestValidator(spec *openapi3.T, ingestToken string) api.MiddlewareFunc {
	return middleware.OapiRequestValidatorWithOptions(spec, &middleware.Options{
		Options: openapi3filter.Options{
			AuthenticationFunc: func(_ context.Context, input *openapi3filter.AuthenticationInput) error {
				if input.SecuritySchemeName != "IngestAuth" {
					return errors.New("unknown security scheme")
				}
				if ingestToken == "" {
					return nil
				}

				header := input.RequestValidationInput.Request.Header.Get("Authorization")
				token := strings.TrimPrefix(header, bearerPrefix)
				if !strings.HasPrefix(header, bearerPrefix) ||
					subtle.ConstantTimeCompare([]byte(token), []byte(ingestToken)) != 1 {
					return errors.New("invalid ingest token")
				}

				return nil
			},
		},
	})
}
