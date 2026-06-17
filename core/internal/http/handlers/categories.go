package handlers

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/oapi-codegen/nullable"
	"go.uber.org/zap"

	"finance/generated/api"
	"finance/internal/entities"
	categoryrepository "finance/internal/repositories/category_repository"
)

func (s Server) ListCategories(ctx context.Context, request api.ListCategoriesRequestObject) (api.ListCategoriesResponseObject, error) {
	var typ *entities.CategoryType
	if request.Params.Type != nil {
		t := entities.CategoryType(*request.Params.Type)
		typ = &t
	}
	includeArchived := request.Params.IncludeArchived != nil && *request.Params.IncludeArchived

	cats, err := s.categories.List(ctx, typ, includeArchived)
	if err != nil {
		return nil, err
	}

	out := make([]api.Category, len(cats))
	for i, c := range cats {
		out[i] = toCategory(c)
	}

	return api.ListCategories200JSONResponse{Categories: out}, nil
}

func (s Server) CreateCategory(ctx context.Context, request api.CreateCategoryRequestObject) (api.CreateCategoryResponseObject, error) {
	if request.Body == nil {
		return api.CreateCategory400JSONResponse{BadRequestJSONResponse: badRequest("empty body")}, nil
	}
	body := request.Body

	cat := entities.Category{
		Name:     body.Name,
		Type:     entities.CategoryType(body.Type),
		ParentID: body.ParentId,
		Icon:     body.Icon,
		Color:    body.Color,
	}

	if cat.ParentID != nil {
		if resp := s.validateParent(ctx, *cat.ParentID, cat.Type); resp != nil {
			return resp, nil
		}
	}

	if err := s.categories.Create(ctx, &cat); err != nil {
		s.logger.Error("create category", zap.Error(err))

		return nil, err
	}

	return api.CreateCategory201JSONResponse(toCategory(cat)), nil
}

// validateParent enforces the two-level tree and matching type. It returns a
// 400 response object when invalid, or nil when the parent is acceptable.
func (s Server) validateParent(ctx context.Context, parentID uuid.UUID, childType entities.CategoryType) api.CreateCategoryResponseObject {
	parent, err := s.categories.Get(ctx, parentID)
	if errors.Is(err, categoryrepository.ErrNotFound) {
		return api.CreateCategory400JSONResponse{BadRequestJSONResponse: badRequest("parent category not found")}
	}
	if err != nil {
		s.logger.Error("load parent category", zap.Error(err))

		return api.CreateCategory400JSONResponse{BadRequestJSONResponse: badRequest("invalid parent category")}
	}
	if parent.Type != childType {
		return api.CreateCategory400JSONResponse{BadRequestJSONResponse: badRequest("parent category has a different type")}
	}
	if parent.ParentID != nil {
		return api.CreateCategory400JSONResponse{BadRequestJSONResponse: badRequest("categories support only two levels")}
	}

	return nil
}

func (s Server) GetCategory(ctx context.Context, request api.GetCategoryRequestObject) (api.GetCategoryResponseObject, error) {
	cat, err := s.categories.Get(ctx, request.Id)
	if errors.Is(err, categoryrepository.ErrNotFound) {
		return api.GetCategory404JSONResponse{NotFoundJSONResponse: notFound("category not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	return api.GetCategory200JSONResponse(toCategory(*cat)), nil
}

func (s Server) UpdateCategory(ctx context.Context, request api.UpdateCategoryRequestObject) (api.UpdateCategoryResponseObject, error) {
	cat, err := s.categories.Get(ctx, request.Id)
	if errors.Is(err, categoryrepository.ErrNotFound) {
		return api.UpdateCategory404JSONResponse{NotFoundJSONResponse: notFound("category not found")}, nil
	}
	if err != nil {
		return nil, err
	}

	body := request.Body
	if body == nil {
		return api.UpdateCategory400JSONResponse{BadRequestJSONResponse: badRequest("empty body")}, nil
	}

	if body.Name != nil {
		if *body.Name == "" {
			return api.UpdateCategory400JSONResponse{BadRequestJSONResponse: badRequest("name must not be empty")}, nil
		}
		cat.Name = *body.Name
	}
	if body.Icon != nil {
		cat.Icon = body.Icon
	}
	if body.Color != nil {
		cat.Color = body.Color
	}
	if body.Archived != nil {
		cat.Archived = *body.Archived
	}

	if err = s.categories.Update(ctx, cat); err != nil {
		s.logger.Error("update category", zap.Error(err))

		return nil, err
	}

	return api.UpdateCategory200JSONResponse(toCategory(*cat)), nil
}

func (s Server) DeleteCategory(ctx context.Context, request api.DeleteCategoryRequestObject) (api.DeleteCategoryResponseObject, error) {
	err := s.categories.Delete(ctx, request.Id)
	switch {
	case errors.Is(err, categoryrepository.ErrNotFound):
		return api.DeleteCategory404JSONResponse{NotFoundJSONResponse: notFound("category not found")}, nil
	case errors.Is(err, categoryrepository.ErrInUse):
		return api.DeleteCategory409JSONResponse{Error: "category has subcategories or transactions; archive it instead"}, nil
	case err != nil:
		return nil, err
	}

	return api.DeleteCategory204Response{}, nil
}

func (s Server) SuggestIcons(ctx context.Context, request api.SuggestIconsRequestObject) (api.SuggestIconsResponseObject, error) {
	if request.Body == nil || request.Body.Name == "" {
		return api.SuggestIcons400JSONResponse{BadRequestJSONResponse: badRequest("name is required")}, nil
	}

	icons, err := s.icons.Suggest(ctx, request.Body.Name, entities.CategoryType(request.Body.Type))
	if err != nil {
		s.logger.Error("suggest icons", zap.Error(err))
		icons = nil
	}
	if icons == nil {
		icons = []string{}
	}

	return api.SuggestIcons200JSONResponse{Icons: icons}, nil
}

func toCategory(c entities.Category) api.Category {
	out := api.Category{
		Id:        c.ID,
		Name:      c.Name,
		Type:      api.CategoryType(c.Type),
		Archived:  c.Archived,
		CreatedAt: c.CreatedAt,
	}

	if c.ParentID != nil {
		out.ParentId = nullable.NewNullableWithValue(*c.ParentID)
	}
	if c.Icon != nil {
		out.Icon = nullable.NewNullableWithValue(*c.Icon)
	}
	if c.Color != nil {
		out.Color = nullable.NewNullableWithValue(*c.Color)
	}

	return out
}
