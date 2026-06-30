// Package mcp wraps the modelcontextprotocol go-sdk (options pattern) so the
// rest of the app builds an MCP server and Streamable-HTTP handler without
// importing the SDK directly.
package mcp

import (
	"net/http"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Re-exported SDK types so internal packages depend only on this wrapper.
type (
	Tool            = sdk.Tool
	CallToolRequest = sdk.CallToolRequest
	CallToolResult  = sdk.CallToolResult
	Content         = sdk.Content
	TextContent     = sdk.TextContent
)

// Server is a thin wrapper around the SDK server plus its HTTP transport.
type Server struct {
	srv *sdk.Server
}

func New(opts ...Option) *Server {
	c := config{name: "mcp", version: "0.0.0"}
	for _, opt := range opts {
		opt(&c)
	}

	return &Server{
		srv: sdk.NewServer(&sdk.Implementation{Name: c.name, Version: c.version}, nil),
	}
}

// Handler serves the server over the Streamable-HTTP transport. The same server
// instance is shared across requests.
func (s *Server) Handler() http.Handler {
	return sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return s.srv }, nil)
}

// AddTool registers a typed tool. In/Out drive the auto-generated JSON schema.
func AddTool[In, Out any](s *Server, t *Tool, h sdk.ToolHandlerFor[In, Out]) {
	sdk.AddTool(s.srv, t, h)
}
