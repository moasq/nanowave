package revenuecatserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Run starts the RevenueCat MCP server over stdio.
// It blocks until the client disconnects or the context is cancelled.
func Run(ctx context.Context) error {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "revenuecat",
			Version: "v1.0.0",
		},
		nil,
	)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_products",
		Description: "List all products in the RevenueCat project. Returns product IDs, store identifiers, types, and display names.",
	}, handleListProducts)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_product",
		Description: "Create a new product in RevenueCat. Requires store_identifier (App Store product ID), app_id (RevenueCat app ID), and type (subscription or one_time).",
	}, handleCreateProduct)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_entitlements",
		Description: "List all entitlements in the RevenueCat project. Returns entitlement IDs, lookup keys, and associated products.",
	}, handleListEntitlements)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_entitlement",
		Description: "Create a new entitlement in RevenueCat. Entitlements represent access levels (e.g. premium). Requires lookup_key and display_name.",
	}, handleCreateEntitlement)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "attach_products_to_entitlement",
		Description: "Attach one or more products to an entitlement. When a customer purchases any attached product, they gain this entitlement.",
	}, handleAttachProductsToEntitlement)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_offerings",
		Description: "List all offerings in the RevenueCat project. Offerings are the containers shown to users with available purchase options.",
	}, handleListOfferings)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_offering",
		Description: "Create a new offering in RevenueCat. Offerings organize products into groups for display (e.g. default offering with monthly and yearly options).",
	}, handleCreateOffering)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_package",
		Description: "Create a new package within an offering. Packages represent individual purchase options within an offering (e.g. monthly, yearly).",
	}, handleCreatePackage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "attach_product_to_package",
		Description: "Attach a product to a package. This links a specific App Store product to a package within an offering.",
	}, handleAttachProductToPackage)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_public_api_keys",
		Description: "Get the public API keys for a RevenueCat app. Returns the SDK keys (appl_ for production, test_ for sandbox) used to initialize the SDK.",
	}, handleGetPublicAPIKeys)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_apps",
		Description: "List all apps in the RevenueCat project. Returns app IDs, names, types, and bundle IDs.",
	}, handleListApps)

	return server.Run(ctx, &mcp.StdioTransport{})
}
