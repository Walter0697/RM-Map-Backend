package middleware

import (
	"context"
	"mapmarker/backend/database/dbmodel"
)

var apiKeyCtxKey = &contextKey{"api-key"}

func APIKeyForContext(ctx context.Context) *dbmodel.APIKey {
	raw, _ := ctx.Value(apiKeyCtxKey).(*dbmodel.APIKey)
	return raw
}

func AddAPIKeyToContext(ctx context.Context, apiKey *dbmodel.APIKey) context.Context {
	return context.WithValue(ctx, apiKeyCtxKey, apiKey)
}
