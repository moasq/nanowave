package revenuecatserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type textOutput struct {
	Message string `json:"message"`
}

// --- list_products ---

type listProductsInput struct{}

func handleListProducts(ctx context.Context, req *mcp.CallToolRequest, input listProductsInput) (*mcp.CallToolResult, textOutput, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	raw, err := c.listProducts(ctx)
	if err != nil {
		return nil, textOutput{}, err
	}
	return nil, textOutput{Message: string(raw)}, nil
}

// --- create_product ---

type createProductInput struct {
	StoreIdentifier string `json:"store_identifier" jsonschema:"The App Store product identifier (e.g. premium_monthly)"`
	AppID           string `json:"app_id" jsonschema:"The RevenueCat app ID to associate this product with"`
	Type            string `json:"type" jsonschema:"Product type: subscription, one_time, consumable, non_consumable"`
	DisplayName     string `json:"display_name" jsonschema:"Human-readable display name for the product"`
	Title           string `json:"title" jsonschema:"User-facing title (required for Test Store products)"`
	Duration        string `json:"duration" jsonschema:"Subscription duration for Test Store: P1W, P1M, P2M, P3M, P6M, P1Y"`
}

func handleCreateProduct(ctx context.Context, req *mcp.CallToolRequest, input createProductInput) (*mcp.CallToolResult, textOutput, error) {
	if input.StoreIdentifier == "" {
		return nil, textOutput{}, fmt.Errorf("store_identifier is required")
	}
	if input.AppID == "" {
		return nil, textOutput{}, fmt.Errorf("app_id is required")
	}
	if input.Type == "" {
		return nil, textOutput{}, fmt.Errorf("type is required (subscription, one_time, consumable, non_consumable)")
	}
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	pi := productInput{
		StoreIdentifier: input.StoreIdentifier,
		AppID:           input.AppID,
		Type:            input.Type,
		DisplayName:     input.DisplayName,
		Title:           input.Title,
	}
	if input.Duration != "" {
		pi.Subscription = &productSubscriptionInput{Duration: input.Duration}
	}
	raw, err := c.createProduct(ctx, pi)
	if err != nil {
		return nil, textOutput{}, err
	}
	return nil, textOutput{Message: string(raw)}, nil
}

// --- list_entitlements ---

type listEntitlementsInput struct{}

func handleListEntitlements(ctx context.Context, req *mcp.CallToolRequest, input listEntitlementsInput) (*mcp.CallToolResult, textOutput, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	raw, err := c.listEntitlements(ctx)
	if err != nil {
		return nil, textOutput{}, err
	}
	return nil, textOutput{Message: string(raw)}, nil
}

// --- create_entitlement ---

type createEntitlementInput struct {
	LookupKey   string `json:"lookup_key" jsonschema:"Unique key for the entitlement (e.g. premium)"`
	DisplayName string `json:"display_name" jsonschema:"Human-readable display name"`
}

func handleCreateEntitlement(ctx context.Context, req *mcp.CallToolRequest, input createEntitlementInput) (*mcp.CallToolResult, textOutput, error) {
	if input.LookupKey == "" {
		return nil, textOutput{}, fmt.Errorf("lookup_key is required")
	}
	if input.DisplayName == "" {
		input.DisplayName = input.LookupKey
	}
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	raw, err := c.createEntitlement(ctx, input.LookupKey, input.DisplayName)
	if err != nil {
		return nil, textOutput{}, err
	}
	return nil, textOutput{Message: string(raw)}, nil
}

// --- attach_products_to_entitlement ---

type attachProductsToEntitlementInput struct {
	EntitlementID string   `json:"entitlement_id" jsonschema:"The entitlement ID to attach products to"`
	ProductIDs    []string `json:"product_ids" jsonschema:"Array of product IDs to attach"`
}

func handleAttachProductsToEntitlement(ctx context.Context, req *mcp.CallToolRequest, input attachProductsToEntitlementInput) (*mcp.CallToolResult, textOutput, error) {
	if input.EntitlementID == "" {
		return nil, textOutput{}, fmt.Errorf("entitlement_id is required")
	}
	if len(input.ProductIDs) == 0 {
		return nil, textOutput{}, fmt.Errorf("at least one product_id is required")
	}
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	if err := c.attachProductsToEntitlement(ctx, input.EntitlementID, input.ProductIDs); err != nil {
		return nil, textOutput{}, err
	}
	return nil, textOutput{Message: fmt.Sprintf("Attached %d product(s) to entitlement %s", len(input.ProductIDs), input.EntitlementID)}, nil
}

// --- list_offerings ---

type listOfferingsInput struct{}

func handleListOfferings(ctx context.Context, req *mcp.CallToolRequest, input listOfferingsInput) (*mcp.CallToolResult, textOutput, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	raw, err := c.listOfferings(ctx)
	if err != nil {
		return nil, textOutput{}, err
	}
	return nil, textOutput{Message: string(raw)}, nil
}

// --- create_offering ---

type createOfferingInput struct {
	LookupKey   string `json:"lookup_key" jsonschema:"Unique key for the offering (e.g. default)"`
	DisplayName string `json:"display_name" jsonschema:"Human-readable display name"`
}

func handleCreateOffering(ctx context.Context, req *mcp.CallToolRequest, input createOfferingInput) (*mcp.CallToolResult, textOutput, error) {
	if input.LookupKey == "" {
		return nil, textOutput{}, fmt.Errorf("lookup_key is required")
	}
	if input.DisplayName == "" {
		input.DisplayName = input.LookupKey
	}
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	raw, err := c.createOffering(ctx, input.LookupKey, input.DisplayName)
	if err != nil {
		return nil, textOutput{}, err
	}
	return nil, textOutput{Message: string(raw)}, nil
}

// --- create_package ---

type createPackageInput struct {
	OfferingID  string `json:"offering_id" jsonschema:"The offering ID to add this package to"`
	LookupKey   string `json:"lookup_key" jsonschema:"Unique key for the package (e.g. premium_monthly)"`
	DisplayName string `json:"display_name" jsonschema:"Human-readable display name"`
	Position    int    `json:"position" jsonschema:"Display position (1-based)"`
}

func handleCreatePackage(ctx context.Context, req *mcp.CallToolRequest, input createPackageInput) (*mcp.CallToolResult, textOutput, error) {
	if input.OfferingID == "" {
		return nil, textOutput{}, fmt.Errorf("offering_id is required")
	}
	if input.LookupKey == "" {
		return nil, textOutput{}, fmt.Errorf("lookup_key is required")
	}
	if input.DisplayName == "" {
		input.DisplayName = input.LookupKey
	}
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	raw, err := c.createPackage(ctx, input.OfferingID, input.LookupKey, input.DisplayName, input.Position)
	if err != nil {
		return nil, textOutput{}, err
	}
	return nil, textOutput{Message: string(raw)}, nil
}

// --- attach_product_to_package ---

type attachProductToPackageInput struct {
	PackageID           string `json:"package_id" jsonschema:"The package ID to attach the product to"`
	ProductID           string `json:"product_id" jsonschema:"The product ID to attach"`
	EligibilityCriteria string `json:"eligibility_criteria" jsonschema:"Eligibility criteria: all, google_sdk_lt_6, google_sdk_ge_6. Defaults to all."`
}

func handleAttachProductToPackage(ctx context.Context, req *mcp.CallToolRequest, input attachProductToPackageInput) (*mcp.CallToolResult, textOutput, error) {
	if input.PackageID == "" {
		return nil, textOutput{}, fmt.Errorf("package_id is required")
	}
	if input.ProductID == "" {
		return nil, textOutput{}, fmt.Errorf("product_id is required")
	}
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	if err := c.attachProductToPackage(ctx, input.PackageID, input.ProductID, input.EligibilityCriteria); err != nil {
		return nil, textOutput{}, err
	}
	return nil, textOutput{Message: fmt.Sprintf("Attached product %s to package %s", input.ProductID, input.PackageID)}, nil
}

// --- get_public_api_keys ---

type getPublicAPIKeysInput struct {
	AppID string `json:"app_id" jsonschema:"The RevenueCat app ID to get public keys for"`
}

func handleGetPublicAPIKeys(ctx context.Context, req *mcp.CallToolRequest, input getPublicAPIKeysInput) (*mcp.CallToolResult, textOutput, error) {
	if input.AppID == "" {
		return nil, textOutput{}, fmt.Errorf("app_id is required")
	}
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	raw, err := c.getPublicAPIKeys(ctx, input.AppID)
	if err != nil {
		return nil, textOutput{}, err
	}
	return nil, textOutput{Message: string(raw)}, nil
}

// --- list_apps ---

type listAppsInput struct{}

func handleListApps(ctx context.Context, req *mcp.CallToolRequest, input listAppsInput) (*mcp.CallToolResult, textOutput, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	raw, err := c.listApps(ctx)
	if err != nil {
		return nil, textOutput{}, err
	}
	return nil, textOutput{Message: string(raw)}, nil
}
