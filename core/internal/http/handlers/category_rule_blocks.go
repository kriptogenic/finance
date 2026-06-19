package handlers

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"finance/generated/api"
	"finance/internal/entities"
)

// Blocks are category_rules with no category (NULL category_id); they never route
// but suppress the "add a rule?" prompt for an ambiguous merchant.

func (s Server) ListCategoryRuleBlocks(ctx context.Context, _ api.ListCategoryRuleBlocksRequestObject) (api.ListCategoryRuleBlocksResponseObject, error) {
	blocks, err := s.categoryRules.ListBlocked(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]api.CategoryRuleBlock, len(blocks))
	for i, b := range blocks {
		out[i] = toCategoryRuleBlock(b)
	}

	return api.ListCategoryRuleBlocks200JSONResponse{Blocks: out}, nil
}

func (s Server) CreateCategoryRuleBlock(ctx context.Context, request api.CreateCategoryRuleBlockRequestObject) (api.CreateCategoryRuleBlockResponseObject, error) {
	if request.Body == nil || strings.TrimSpace(request.Body.Merchant) == "" {
		return api.CreateCategoryRuleBlock400JSONResponse{BadRequestJSONResponse: badRequest("merchant is required")}, nil
	}

	block, err := s.categoryRules.AddBlock(ctx, request.Body.Merchant)
	if err != nil {
		s.logger.Error("block merchant rule", zap.Error(err))

		return nil, err
	}

	return api.CreateCategoryRuleBlock201JSONResponse(toCategoryRuleBlock(block)), nil
}

func toCategoryRuleBlock(r entities.CategoryRule) api.CategoryRuleBlock {
	return api.CategoryRuleBlock{Merchant: r.Pattern, CreatedAt: r.CreatedAt}
}
