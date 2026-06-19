// Package categorysuggest ranks a user's existing categories for an
// uncategorized transaction's merchant, using an LLM. It mirrors iconsuggest:
// disabled (no-op) when no API key is configured.
package categorysuggest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"finance/config"
	"finance/pkg/anthropic"
)

const maxSuggestions = 6

type Suggester interface {
	// Suggest ranks candidate category names for a merchant, most relevant first.
	// It only ever returns names drawn from candidates.
	Suggest(ctx context.Context, merchant string, candidates []string) ([]string, error)
}

func New(cfg *config.Anthropic, logger *zap.Logger) Suggester {
	if cfg.APIKey == "" {
		logger.Warn("category suggestions disabled: ANTHROPIC_API_KEY not set")

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

func (disabled) Suggest(context.Context, string, []string) ([]string, error) {
	return nil, nil
}

type llm struct {
	client *anthropic.Client
	logger *zap.Logger
}

const systemPrompt = `You categorize a personal-finance transaction.
You are given a merchant/description and a list of allowed category names.
Return ONLY a JSON array of the most relevant category names, most relevant first,
chosen STRICTLY from the allowed list (copy them verbatim). Return at most 6.
If nothing fits, return []. No prose, no markdown, no code fences.`

func (l *llm) Suggest(ctx context.Context, merchant string, candidates []string) ([]string, error) {
	merchant = strings.TrimSpace(merchant)
	if merchant == "" || len(candidates) == 0 {
		return nil, nil
	}

	list, err := json.Marshal(candidates)
	if err != nil {
		return nil, fmt.Errorf("encode candidates: %w", err)
	}
	prompt := fmt.Sprintf("Merchant: %q\nAllowed categories: %s", merchant, list)

	out, err := l.client.Complete(ctx, systemPrompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("suggest categories: %w", err)
	}

	return parseNames(out, candidates), nil
}

// parseNames extracts the JSON array from the model output (tolerating stray
// prose or code fences) and keeps only names that exist in candidates,
// case-insensitively, deduped, capped — returning the candidates' exact spelling.
func parseNames(out string, candidates []string) []string {
	start := strings.IndexByte(out, '[')
	end := strings.LastIndexByte(out, ']')
	if start < 0 || end <= start {
		return nil
	}

	var raw []string
	if err := json.Unmarshal([]byte(out[start:end+1]), &raw); err != nil {
		return nil
	}

	allowed := make(map[string]string, len(candidates))
	for _, c := range candidates {
		allowed[strings.ToLower(strings.TrimSpace(c))] = c
	}

	seen := make(map[string]struct{}, len(raw))
	names := make([]string, 0, len(raw))
	for _, n := range raw {
		canonical, ok := allowed[strings.ToLower(strings.TrimSpace(n))]
		if !ok {
			continue
		}
		if _, dup := seen[canonical]; dup {
			continue
		}
		seen[canonical] = struct{}{}
		names = append(names, canonical)
		if len(names) >= maxSuggestions {
			break
		}
	}

	return names
}
