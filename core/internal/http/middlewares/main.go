package middlewares

import (
	"github.com/getkin/kin-openapi/openapi3"
	middleware "github.com/oapi-codegen/nethttp-middleware"

	"finance/generated/api"
)

// OpenAPIRequestValidator validates incoming requests against the OpenAPI spec
// (path, query, body schemas) before they reach a handler.
func OpenAPIRequestValidator(spec *openapi3.T) api.MiddlewareFunc {
	return middleware.OapiRequestValidator(spec)
}
