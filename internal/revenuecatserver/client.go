package revenuecatserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const rcAPIBase = "https://api.revenuecat.com/v2"

// revenuecatClient wraps HTTP calls to the RevenueCat REST API v2.
type revenuecatClient struct {
	httpClient *http.Client
	apiKey     string // REVENUECAT_API_KEY (sk_ secret key)
	projectID  string // REVENUECAT_PROJECT_ID
}

// newClientFromEnv reads credentials from environment variables.
func newClientFromEnv() (*revenuecatClient, error) {
	apiKey := os.Getenv("REVENUECAT_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("REVENUECAT_API_KEY is not set")
	}
	projectID := os.Getenv("REVENUECAT_PROJECT_ID")
	if projectID == "" {
		return nil, fmt.Errorf("REVENUECAT_PROJECT_ID is not set")
	}
	return &revenuecatClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiKey:     apiKey,
		projectID:  projectID,
	}, nil
}

func (c *revenuecatClient) doJSON(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	url := rcAPIBase + path

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API %s %s returned %d: %s", method, path, resp.StatusCode, string(respData))
	}

	if len(respData) == 0 {
		return json.RawMessage("{}"), nil
	}
	return json.RawMessage(respData), nil
}

func (c *revenuecatClient) listProducts(ctx context.Context) (json.RawMessage, error) {
	path := fmt.Sprintf("/projects/%s/products", c.projectID)
	return c.doJSON(ctx, http.MethodGet, path, nil)
}

type productInput struct {
	StoreIdentifier string                     `json:"store_identifier"`
	AppID           string                     `json:"app_id"`
	Type            string                     `json:"type"`
	DisplayName     string                     `json:"display_name,omitempty"`
	Title           string                     `json:"title,omitempty"`          // required for Test Store products
	Subscription    *productSubscriptionInput  `json:"subscription,omitempty"`   // required for Test Store subscriptions
}

type productSubscriptionInput struct {
	Duration string `json:"duration"` // ISO 8601: P1W, P1M, P2M, P3M, P6M, P1Y
}

func (c *revenuecatClient) createProduct(ctx context.Context, input productInput) (json.RawMessage, error) {
	path := fmt.Sprintf("/projects/%s/products", c.projectID)
	return c.doJSON(ctx, http.MethodPost, path, input)
}

func (c *revenuecatClient) listEntitlements(ctx context.Context) (json.RawMessage, error) {
	path := fmt.Sprintf("/projects/%s/entitlements", c.projectID)
	return c.doJSON(ctx, http.MethodGet, path, nil)
}

func (c *revenuecatClient) createEntitlement(ctx context.Context, lookupKey, displayName string) (json.RawMessage, error) {
	path := fmt.Sprintf("/projects/%s/entitlements", c.projectID)
	body := map[string]string{
		"lookup_key":   lookupKey,
		"display_name": displayName,
	}
	return c.doJSON(ctx, http.MethodPost, path, body)
}

func (c *revenuecatClient) attachProductsToEntitlement(ctx context.Context, entitlementID string, productIDs []string) error {
	path := fmt.Sprintf("/projects/%s/entitlements/%s/actions/attach_products", c.projectID, entitlementID)
	_, err := c.doJSON(ctx, http.MethodPost, path, map[string]any{"product_ids": productIDs})
	return err
}

func (c *revenuecatClient) listOfferings(ctx context.Context) (json.RawMessage, error) {
	path := fmt.Sprintf("/projects/%s/offerings", c.projectID)
	return c.doJSON(ctx, http.MethodGet, path, nil)
}

func (c *revenuecatClient) createOffering(ctx context.Context, lookupKey, displayName string) (json.RawMessage, error) {
	path := fmt.Sprintf("/projects/%s/offerings", c.projectID)
	body := map[string]string{
		"lookup_key":   lookupKey,
		"display_name": displayName,
	}
	return c.doJSON(ctx, http.MethodPost, path, body)
}

func (c *revenuecatClient) createPackage(ctx context.Context, offeringID, lookupKey, displayName string, position int) (json.RawMessage, error) {
	path := fmt.Sprintf("/projects/%s/offerings/%s/packages", c.projectID, offeringID)
	body := map[string]any{
		"lookup_key":   lookupKey,
		"display_name": displayName,
		"position":     position,
	}
	return c.doJSON(ctx, http.MethodPost, path, body)
}

func (c *revenuecatClient) attachProductToPackage(ctx context.Context, packageID, productID, eligibilityCriteria string) error {
	path := fmt.Sprintf("/projects/%s/packages/%s/actions/attach_products", c.projectID, packageID)
	type productAssoc struct {
		ProductID           string `json:"product_id"`
		EligibilityCriteria string `json:"eligibility_criteria"`
	}
	if eligibilityCriteria == "" {
		eligibilityCriteria = "all"
	}
	body := map[string]any{
		"products": []productAssoc{{ProductID: productID, EligibilityCriteria: eligibilityCriteria}},
	}
	_, err := c.doJSON(ctx, http.MethodPost, path, body)
	return err
}

func (c *revenuecatClient) getPublicAPIKeys(ctx context.Context, appID string) (json.RawMessage, error) {
	path := fmt.Sprintf("/projects/%s/apps/%s/public_api_keys", c.projectID, appID)
	return c.doJSON(ctx, http.MethodGet, path, nil)
}

func (c *revenuecatClient) listApps(ctx context.Context) (json.RawMessage, error) {
	path := fmt.Sprintf("/projects/%s/apps", c.projectID)
	return c.doJSON(ctx, http.MethodGet, path, nil)
}
