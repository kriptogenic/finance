package handlers

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"go.uber.org/zap"

	"finance/generated/api"
	"finance/internal/entities"
)

// allowedPushHosts are the Web Push autopush endpoints of the major browser
// vendors. A subscription endpoint's host must equal one of these or be a
// subdomain of one. This is the SSRF guard: the endpoint is fetched server-side
// by the push sender, so it must point at a known push service, never arbitrary
// (e.g. internal) infrastructure.
var allowedPushHosts = []string{
	"fcm.googleapis.com",        // Chrome / Edge / Chromium (FCM)
	"android.googleapis.com",    // legacy GCM endpoints
	"push.services.mozilla.com", // Firefox autopush (incl. regional subdomains)
	"push.apple.com",            // Safari (web.push.apple.com)
	"notify.windows.com",        // legacy Edge / WNS (*.notify.windows.com)
}

// validatePushEndpoint requires an https URL whose host is a recognized push
// service (see allowedPushHosts). IP literals and any other host are rejected.
func validatePushEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("endpoint is not a valid URL")
	}
	if u.Scheme != "https" {
		return errors.New("endpoint must be an https URL")
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return errors.New("endpoint has no host")
	}
	if !isAllowedPushHost(host) {
		return errors.New("endpoint host is not a recognized push service")
	}

	return nil
}

func isAllowedPushHost(host string) bool {
	for _, allowed := range allowedPushHosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}

	return false
}

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

	if err := validatePushEndpoint(body.Endpoint); err != nil {
		return api.SubscribePush400JSONResponse{BadRequestJSONResponse: badRequest(err.Error())}, nil
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
