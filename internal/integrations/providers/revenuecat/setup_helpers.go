package revenuecat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const rcAPIBase = "https://api.revenuecat.com/v2"

// rcClient wraps HTTP calls to the RevenueCat REST API v2.
type rcClient struct {
	httpClient *http.Client
	secretKey  string // sk_ secret API key
}

func newRCClient(secretKey string) *rcClient {
	return &rcClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		secretKey:  secretKey,
	}
}

func (c *rcClient) doJSON(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
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
	req.Header.Set("Authorization", "Bearer "+c.secretKey)
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

// rcProject represents a RevenueCat project.
type rcProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// rcApp represents a RevenueCat app.
type rcApp struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Type     string     `json:"type"` // "app_store", "test_store", etc.
	AppStore *rcAppStore `json:"app_store,omitempty"`
}

// rcAppStore holds App Store-specific details.
type rcAppStore struct {
	BundleID string `json:"bundle_id"`
}

// BundleID returns the bundle ID if available.
func (a rcApp) BundleID() string {
	if a.AppStore != nil {
		return a.AppStore.BundleID
	}
	return ""
}

// rcPublicKey represents a RevenueCat public API key.
type rcPublicKey struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Environment string `json:"environment"` // "production" or "sandbox"
}

// rcProductInput is the body for creating a product.
type rcProductInput struct {
	StoreIdentifier string                    `json:"store_identifier"`
	AppID           string                    `json:"app_id"`
	Type            string                    `json:"type"` // "subscription", "one_time", "consumable", "non_consumable"
	DisplayName     string                    `json:"display_name,omitempty"`
	Title           string                    `json:"title,omitempty"`           // required for Test Store products
	Subscription    *rcProductSubscriptionInput `json:"subscription,omitempty"`  // required for Test Store subscriptions
}

// rcProductSubscriptionInput holds subscription-specific parameters (Test Store only).
type rcProductSubscriptionInput struct {
	Duration string `json:"duration"` // ISO 8601: P1W, P1M, P2M, P3M, P6M, P1Y
}

// rcProduct represents a created product.
type rcProduct struct {
	ID              string `json:"id"`
	StoreIdentifier string `json:"store_identifier"`
	AppID           string `json:"app_id"`
	DisplayName     string `json:"display_name"`
}

// rcEntitlement represents a RevenueCat entitlement.
type rcEntitlement struct {
	ID          string `json:"id"`
	LookupKey   string `json:"lookup_key"`
	DisplayName string `json:"display_name"`
}

// rcOffering represents a RevenueCat offering.
type rcOffering struct {
	ID          string `json:"id"`
	LookupKey   string `json:"lookup_key"`
	DisplayName string `json:"display_name"`
}

// rcPackage represents a RevenueCat package within an offering.
type rcPackage struct {
	ID          string `json:"id"`
	LookupKey   string `json:"lookup_key"`
	DisplayName string `json:"display_name"`
}

func (c *rcClient) listProjects(ctx context.Context) ([]rcProject, error) {
	raw, err := c.doJSON(ctx, http.MethodGet, "/projects", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Items []rcProject `json:"items"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse projects: %w", err)
	}
	return resp.Items, nil
}

func (c *rcClient) listApps(ctx context.Context, projectID string) ([]rcApp, error) {
	path := fmt.Sprintf("/projects/%s/apps", projectID)
	raw, err := c.doJSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Items []rcApp `json:"items"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse apps: %w", err)
	}
	return resp.Items, nil
}

func (c *rcClient) createApp(ctx context.Context, projectID, name, bundleID string) (*rcApp, error) {
	path := fmt.Sprintf("/projects/%s/apps", projectID)
	body := map[string]any{
		"name": name,
		"type": "app_store",
		"app_store": map[string]string{
			"bundle_id": bundleID,
		},
	}
	raw, err := c.doJSON(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	var app rcApp
	if err := json.Unmarshal(raw, &app); err != nil {
		return nil, fmt.Errorf("parse app: %w", err)
	}
	return &app, nil
}

func (c *rcClient) getPublicAPIKeys(ctx context.Context, projectID, appID string) ([]rcPublicKey, error) {
	path := fmt.Sprintf("/projects/%s/apps/%s/public_api_keys", projectID, appID)
	raw, err := c.doJSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Items []rcPublicKey `json:"items"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse API keys: %w", err)
	}
	return resp.Items, nil
}

func (c *rcClient) createProduct(ctx context.Context, projectID string, p rcProductInput) (*rcProduct, error) {
	path := fmt.Sprintf("/projects/%s/products", projectID)
	raw, err := c.doJSON(ctx, http.MethodPost, path, p)
	if err != nil {
		return nil, err
	}
	var product rcProduct
	if err := json.Unmarshal(raw, &product); err != nil {
		return nil, fmt.Errorf("parse product: %w", err)
	}
	return &product, nil
}

func (c *rcClient) createEntitlement(ctx context.Context, projectID, lookupKey, displayName string) (*rcEntitlement, error) {
	path := fmt.Sprintf("/projects/%s/entitlements", projectID)
	body := map[string]string{
		"lookup_key":   lookupKey,
		"display_name": displayName,
	}
	raw, err := c.doJSON(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	var ent rcEntitlement
	if err := json.Unmarshal(raw, &ent); err != nil {
		return nil, fmt.Errorf("parse entitlement: %w", err)
	}
	return &ent, nil
}

func (c *rcClient) attachProductsToEntitlement(ctx context.Context, projectID, entitlementID string, productIDs []string) error {
	path := fmt.Sprintf("/projects/%s/entitlements/%s/actions/attach_products", projectID, entitlementID)
	_, err := c.doJSON(ctx, http.MethodPost, path, map[string]any{"product_ids": productIDs})
	return err
}

func (c *rcClient) createOffering(ctx context.Context, projectID, lookupKey, displayName string) (*rcOffering, error) {
	path := fmt.Sprintf("/projects/%s/offerings", projectID)
	body := map[string]string{
		"lookup_key":   lookupKey,
		"display_name": displayName,
	}
	raw, err := c.doJSON(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	var off rcOffering
	if err := json.Unmarshal(raw, &off); err != nil {
		return nil, fmt.Errorf("parse offering: %w", err)
	}
	return &off, nil
}

func (c *rcClient) createPackage(ctx context.Context, projectID, offeringID, lookupKey, displayName string, position int) (*rcPackage, error) {
	path := fmt.Sprintf("/projects/%s/offerings/%s/packages", projectID, offeringID)
	body := map[string]any{
		"lookup_key":   lookupKey,
		"display_name": displayName,
		"position":     position,
	}
	raw, err := c.doJSON(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	var pkg rcPackage
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return nil, fmt.Errorf("parse package: %w", err)
	}
	return &pkg, nil
}

func (c *rcClient) attachProductToPackage(ctx context.Context, projectID, packageID, productID string) error {
	path := fmt.Sprintf("/projects/%s/packages/%s/actions/attach_products", projectID, packageID)
	type productAssoc struct {
		ProductID           string `json:"product_id"`
		EligibilityCriteria string `json:"eligibility_criteria"`
	}
	body := map[string]any{
		"products": []productAssoc{{ProductID: productID, EligibilityCriteria: "all"}},
	}
	_, err := c.doJSON(ctx, http.MethodPost, path, body)
	return err
}

func (c *rcClient) listEntitlements(ctx context.Context, projectID string) (json.RawMessage, error) {
	path := fmt.Sprintf("/projects/%s/entitlements", projectID)
	return c.doJSON(ctx, http.MethodGet, path, nil)
}

func (c *rcClient) listProducts(ctx context.Context, projectID string) (json.RawMessage, error) {
	path := fmt.Sprintf("/projects/%s/products", projectID)
	return c.doJSON(ctx, http.MethodGet, path, nil)
}

func (c *rcClient) listOfferings(ctx context.Context, projectID string) (json.RawMessage, error) {
	path := fmt.Sprintf("/projects/%s/offerings", projectID)
	return c.doJSON(ctx, http.MethodGet, path, nil)
}

// findProductByStoreID returns an existing product by store_identifier for a given app, or nil.
func (c *rcClient) findProductByStoreID(ctx context.Context, projectID, appID, storeIdentifier string) *rcProduct {
	path := fmt.Sprintf("/projects/%s/products?app_id=%s", projectID, appID)
	raw, err := c.doJSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil
	}
	var resp struct {
		Items []rcProduct `json:"items"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil
	}
	for _, p := range resp.Items {
		if p.StoreIdentifier == storeIdentifier {
			return &p
		}
	}
	return nil
}

// findProductByDisplayName returns an existing product by display_name for a given app, or nil.
// This is needed because RevenueCat enforces unique display_name per app, so a 409 on product
// creation may be a display_name conflict (not a store_identifier conflict).
func (c *rcClient) findProductByDisplayName(ctx context.Context, projectID, appID, displayName string) *rcProduct {
	path := fmt.Sprintf("/projects/%s/products?app_id=%s", projectID, appID)
	raw, err := c.doJSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil
	}
	var resp struct {
		Items []rcProduct `json:"items"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil
	}
	for _, p := range resp.Items {
		if p.DisplayName == displayName {
			return &p
		}
	}
	return nil
}

// findOfferingByKey returns an existing offering by lookup_key, or nil if not found.
func (c *rcClient) findOfferingByKey(ctx context.Context, projectID, lookupKey string) *rcOffering {
	raw, err := c.listOfferings(ctx, projectID)
	if err != nil {
		return nil
	}
	var resp struct {
		Items []rcOffering `json:"items"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil
	}
	for _, o := range resp.Items {
		if o.LookupKey == lookupKey {
			return &o
		}
	}
	return nil
}

// findEntitlementByKey returns an existing entitlement by lookup_key, or nil if not found.
func (c *rcClient) findEntitlementByKey(ctx context.Context, projectID, lookupKey string) *rcEntitlement {
	raw, err := c.listEntitlements(ctx, projectID)
	if err != nil {
		return nil
	}
	var resp struct {
		Items []rcEntitlement `json:"items"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil
	}
	for _, e := range resp.Items {
		if e.LookupKey == lookupKey {
			return &e
		}
	}
	return nil
}

// findPackageByKey returns an existing package by lookup_key within an offering, or nil if not found.
func (c *rcClient) findPackageByKey(ctx context.Context, projectID, offeringID, lookupKey string) *rcPackage {
	path := fmt.Sprintf("/projects/%s/offerings/%s/packages", projectID, offeringID)
	raw, err := c.doJSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil
	}
	var resp struct {
		Items []rcPackage `json:"items"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil
	}
	for _, p := range resp.Items {
		if p.LookupKey == lookupKey {
			return &p
		}
	}
	return nil
}

// findPackageByDisplayName returns an existing package by display_name within an offering, or nil.
func (c *rcClient) findPackageByDisplayName(ctx context.Context, projectID, offeringID, displayName string) *rcPackage {
	path := fmt.Sprintf("/projects/%s/offerings/%s/packages", projectID, offeringID)
	raw, err := c.doJSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil
	}
	var resp struct {
		Items []rcPackage `json:"items"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil
	}
	for _, p := range resp.Items {
		if p.DisplayName == displayName {
			return &p
		}
	}
	return nil
}

func (c *rcClient) validateConnection(ctx context.Context, projectID string) error {
	_, err := c.listEntitlements(ctx, projectID)
	return err
}
