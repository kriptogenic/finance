package audit

import (
	"encoding/json"
	"time"

	"finance/config"
	mcpwrap "finance/pkg/mcp"
)

const version = "0.1.0"

// NewMCPServer builds the finance-audit MCP server with every tool registered.
func NewMCPServer(svc *Service, app *config.App) *mcpwrap.Server {
	srv := mcpwrap.New(mcpwrap.Name(app.Name+"-audit"), mcpwrap.Version(version))
	svc.register(srv)

	return srv
}

// result renders an output value as a JSON text block plus permissive
// structured content. Out is typed as `any` so the SDK skips output-schema
// validation, which otherwise rejects money.Money/uuid.UUID (their JSON shape
// differs from the Go struct the schema generator sees).
func result(out any) (*mcpwrap.CallToolResult, any, error) {
	b, err := json.Marshal(out)
	if err != nil {
		return nil, nil, err
	}

	return &mcpwrap.CallToolResult{
		Content: []mcpwrap.Content{&mcpwrap.TextContent{Text: string(b)}},
	}, out, nil
}

// parseDate reads a YYYY-MM-DD date; empty returns the zero time and false.
func parseDate(s string) (time.Time, bool, error) {
	if s == "" {
		return time.Time{}, false, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, false, err
	}

	return t, true, nil
}

// dateWindow resolves optional from/to bounds to a concrete [from, to] window,
// mirroring handlers.dateRange: omitted bounds mean "all time up to now".
func dateWindow(fromStr, toStr string) (time.Time, time.Time, error) {
	from := time.Unix(0, 0)
	to := time.Now()

	if t, ok, err := parseDate(fromStr); err != nil {
		return from, to, err
	} else if ok {
		from = t
	}

	if t, ok, err := parseDate(toStr); err != nil {
		return from, to, err
	} else if ok {
		to = t
	}

	return from, to, nil
}
