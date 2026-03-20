package revenuecatserver

import (
	"context"
	"encoding/json"
)

// Exported wrappers for CLI tool invocation.
// These assume env vars are already set (REVENUECAT_API_KEY, REVENUECAT_PROJECT_ID).

func ListProducts(ctx context.Context) (json.RawMessage, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, err
	}
	return c.listProducts(ctx)
}

func CreateProduct(ctx context.Context, storeIdentifier, appID, productType string) (json.RawMessage, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, err
	}
	return c.createProduct(ctx, productInput{
		StoreIdentifier: storeIdentifier,
		AppID:           appID,
		Type:            productType,
	})
}

func ListEntitlements(ctx context.Context) (json.RawMessage, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, err
	}
	return c.listEntitlements(ctx)
}

func CreateEntitlement(ctx context.Context, lookupKey, displayName string) (json.RawMessage, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, err
	}
	return c.createEntitlement(ctx, lookupKey, displayName)
}

func GetPublicAPIKeys(ctx context.Context, appID string) (json.RawMessage, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, err
	}
	return c.getPublicAPIKeys(ctx, appID)
}

func ListApps(ctx context.Context) (json.RawMessage, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, err
	}
	return c.listApps(ctx)
}

func ListOfferings(ctx context.Context) (json.RawMessage, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, err
	}
	return c.listOfferings(ctx)
}

func CreateOffering(ctx context.Context, lookupKey, displayName string, isCurrent bool) (json.RawMessage, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, err
	}
	return c.createOffering(ctx, lookupKey, displayName)
}
