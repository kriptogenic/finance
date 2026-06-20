package handlers

import (
	"context"

	"go.uber.org/zap"

	"finance/generated/api"
	"finance/internal/entities"
)

func (s Server) GetPushPublicKey(_ context.Context, _ api.GetPushPublicKeyRequestObject) (api.GetPushPublicKeyResponseObject, error) {
	return api.GetPushPublicKey200JSONResponse{Key: s.vapidPublicKey}, nil
}

func (s Server) SubscribePush(ctx context.Context, request api.SubscribePushRequestObject) (api.SubscribePushResponseObject, error) {
	if request.Body == nil {
		return api.SubscribePush400JSONResponse{BadRequestJSONResponse: badRequest("empty body")}, nil
	}
	body := request.Body
	if body.Endpoint == "" || body.Keys.P256dh == "" || body.Keys.Auth == "" {
		return api.SubscribePush400JSONResponse{BadRequestJSONResponse: badRequest("endpoint and keys are required")}, nil
	}

	sub := entities.PushSubscription{
		Endpoint: body.Endpoint,
		P256dh:   body.Keys.P256dh,
		Auth:     body.Keys.Auth,
	}
	if err := s.push.Upsert(ctx, &sub); err != nil {
		s.logger.Error("upsert push subscription", zap.Error(err))

		return nil, err
	}

	return api.SubscribePush204Response{}, nil
}

func (s Server) UnsubscribePush(ctx context.Context, request api.UnsubscribePushRequestObject) (api.UnsubscribePushResponseObject, error) {
	if request.Params.Endpoint == "" {
		return api.UnsubscribePush400JSONResponse{BadRequestJSONResponse: badRequest("endpoint is required")}, nil
	}
	if err := s.push.Delete(ctx, request.Params.Endpoint); err != nil {
		s.logger.Error("delete push subscription", zap.Error(err))

		return nil, err
	}

	return api.UnsubscribePush204Response{}, nil
}
