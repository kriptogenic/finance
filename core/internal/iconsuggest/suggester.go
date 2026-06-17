package iconsuggest

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"go.uber.org/zap"

	"finance/config"
	"finance/internal/entities"
	"finance/pkg/anthropic"
)

const maxIcons = 8

type Suggester interface {
	Suggest(ctx context.Context, name string, typ entities.CategoryType) ([]string, error)
}

func New(cfg *config.Anthropic, logger *zap.Logger) Suggester {
	if cfg.APIKey == "" {
		logger.Warn("category icon suggestions disabled: ANTHROPIC_API_KEY not set")

		return disabled{}
	}

	return &llm{
		client: anthropic.New(cfg.APIKey,
			anthropic.Model(cfg.Model),
			anthropic.MaxTokens(256),
		),
		logger: logger,
	}
}

type disabled struct{}

func (disabled) Suggest(context.Context, string, entities.CategoryType) ([]string, error) {
	return nil, nil
}

type llm struct {
	client *anthropic.Client
	logger *zap.Logger
}

const systemPrompt = `You suggest icons for a personal-finance category.
Return ONLY a JSON array of up to 8 Tabler Icons names (https://tabler.io/icons) in kebab-case, most relevant first.
No prose, no markdown, no code fences. Use only real Tabler icon names.
Example: ["shopping-cart","basket","meat","carrot","receipt"]`

func (l *llm) Suggest(ctx context.Context, name string, typ entities.CategoryType) ([]string, error) {
	prompt := fmt.Sprintf("Category name: %q\nType: %s", name, typ)

	out, err := l.client.Complete(ctx, systemPrompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("suggest icons: %w", err)
	}

	return parseIcons(out), nil
}

var nameRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// parseIcons extracts the JSON array of icon names from the model output,
// tolerating stray prose or code fences, and keeps only well-formed names.
func parseIcons(out string) []string {
	start := strings.IndexByte(out, '[')
	end := strings.LastIndexByte(out, ']')
	if start < 0 || end <= start {
		return nil
	}

	var raw []string
	if err := json.Unmarshal([]byte(out[start:end+1]), &raw); err != nil {
		return nil
	}

	seen := make(map[string]struct{}, len(raw))
	icons := make([]string, 0, len(raw))
	for _, n := range raw {
		n = strings.TrimSpace(strings.ToLower(n))
		if !nameRe.MatchString(n) {
			continue
		}
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		icons = append(icons, n)
		if len(icons) >= maxIcons {
			break
		}
	}

	return icons
}
