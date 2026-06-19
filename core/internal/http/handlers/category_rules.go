package handlers

import (
	"context"
	"errors"

	"go.uber.org/zap"

	"finance/generated/api"
	"finance/internal/entities"
	categoryrepository "finance/internal/repositories/category_repository"
	categoryrulerepository "finance/internal/repositories/category_rule_repository"
)

func (s Server) ListCategoryRules(ctx context.Context, _ api.ListCategoryRulesRequestObject) (api.ListCategoryRulesResponseObject, error) {
	rules, err := s.categoryRules.List(ctx)
	if err != nil {
		return nil, err
	}

	names, err := s.categoryNames(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]api.CategoryRule, len(rules))
	for i, r := range rules {
		out[i] = toCategoryRule(r, names[*r.CategoryID])
	}

	return api.ListCategoryRules200JSONResponse{Rules: out}, nil
}

func (s Server) CreateCategoryRule(ctx context.Context, request api.CreateCategoryRuleRequestObject) (api.CreateCategoryRuleResponseObject, error) {
	if request.Body == nil {
		return api.CreateCategoryRule400JSONResponse{BadRequestJSONResponse: badRequest("empty body")}, nil
	}
	body := request.Body
	if body.Pattern == "" {
		return api.CreateCategoryRule400JSONResponse{BadRequestJSONResponse: badRequest("pattern must not be empty")}, nil
	}

	cat, err := s.categories.Get(ctx, body.CategoryId)
	if errors.Is(err, categoryrepository.ErrNotFound) {
		return api.CreateCategoryRule400JSONResponse{BadRequestJSONResponse: badRequest("category not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	rule := entities.CategoryRule{Pattern: body.Pattern, CategoryID: &body.CategoryId}
	if err = s.categoryRules.Create(ctx, &rule); err != nil {
		s.logger.Error("create category rule", zap.Error(err))

		return nil, err
	}

	return api.CreateCategoryRule201JSONResponse(toCategoryRule(rule, cat.Name)), nil
}

func (s Server) UpdateCategoryRule(ctx context.Context, request api.UpdateCategoryRuleRequestObject) (api.UpdateCategoryRuleResponseObject, error) {
	rule, err := s.categoryRules.Get(ctx, request.Id)
	if errors.Is(err, categoryrulerepository.ErrNotFound) {
		return api.UpdateCategoryRule404JSONResponse{NotFoundJSONResponse: notFound("category rule not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	body := request.Body
	if body == nil {
		return api.UpdateCategoryRule400JSONResponse{BadRequestJSONResponse: badRequest("empty body")}, nil
	}
	if body.Pattern != nil {
		if *body.Pattern == "" {
			return api.UpdateCategoryRule400JSONResponse{BadRequestJSONResponse: badRequest("pattern must not be empty")}, nil
		}
		rule.Pattern = *body.Pattern
	}
	if body.CategoryId != nil {
		_, cerr := s.categories.Get(ctx, *body.CategoryId)
		if errors.Is(cerr, categoryrepository.ErrNotFound) {
			return api.UpdateCategoryRule400JSONResponse{BadRequestJSONResponse: badRequest("category not found")}, nil
		}
		if cerr != nil {
			return nil, cerr
		}
		rule.CategoryID = body.CategoryId
	}

	if err = s.categoryRules.Update(ctx, rule); err != nil {
		s.logger.Error("update category rule", zap.Error(err))

		return nil, err
	}

	name := ""
	if rule.CategoryID != nil {
		cat, cerr := s.categories.Get(ctx, *rule.CategoryID)
		if cerr != nil {
			return nil, cerr
		}
		name = cat.Name
	}

	return api.UpdateCategoryRule200JSONResponse(toCategoryRule(*rule, name)), nil
}

func (s Server) DeleteCategoryRule(ctx context.Context, request api.DeleteCategoryRuleRequestObject) (api.DeleteCategoryRuleResponseObject, error) {
	err := s.categoryRules.Delete(ctx, request.Id)
	if errors.Is(err, categoryrulerepository.ErrNotFound) {
		return api.DeleteCategoryRule404JSONResponse{NotFoundJSONResponse: notFound("category rule not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.DeleteCategoryRule204Response{}, nil
}

func toCategoryRule(r entities.CategoryRule, categoryName string) api.CategoryRule {
	rule := api.CategoryRule{
		Id:           r.ID,
		Pattern:      r.Pattern,
		CategoryName: categoryName,
		CreatedAt:    r.CreatedAt,
	}
	if r.CategoryID != nil {
		rule.CategoryId = *r.CategoryID
	}

	return rule
}
