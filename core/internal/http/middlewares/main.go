package middlewares

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	middleware "github.com/oapi-codegen/nethttp-middleware"

	"finance/config"
	"finance/generated/api"
)

const bearerPrefix = "Bearer "

func OpenAPIRequestValidator(spec *openapi3.T, auth *config.Auth, ingest *config.Ingest) api.MiddlewareFunc {
	return middleware.OapiRequestValidatorWithOptions(spec, &middleware.Options{
		Options: openapi3filter.Options{
			AuthenticationFunc: func(_ context.Context, input *openapi3filter.AuthenticationInput) error {
				req := input.RequestValidationInput.Request
				switch input.SecuritySchemeName {
				case "BasicAuth":
					return authenticateBasic(req, auth)
				case "IngestAuth":
					return authenticateBearer(req, ingest.Token)
				default:
					return errors.New("unknown security scheme")
				}
			},
		},
	})
}

func authenticateBasic(req *http.Request, auth *config.Auth) error {
	username, password, ok := req.BasicAuth()
	if !ok {
		return errors.New("missing basic auth credentials")
	}
	if !secureCompare(username, auth.Username) || !secureCompare(password, auth.Password) {
		return errors.New("invalid username or password")
	}

	return nil
}

func authenticateBearer(req *http.Request, token string) error {
	if token == "" {
		return nil
	}

	header := req.Header.Get("Authorization")
	got := strings.TrimPrefix(header, bearerPrefix)
	if !strings.HasPrefix(header, bearerPrefix) ||
		subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
		return errors.New("invalid ingest token")
	}

	return nil
}

func secureCompare(a, b string) bool {
	ah := sha256.Sum256([]byte(a))
	bh := sha256.Sum256([]byte(b))

	return subtle.ConstantTimeCompare(ah[:], bh[:]) == 1
}
