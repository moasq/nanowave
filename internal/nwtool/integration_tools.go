package nwtool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moasq/nanowave/internal/integrations"
	"github.com/moasq/nanowave/internal/integrations/providers"
	"github.com/moasq/nanowave/internal/supabaseserver"
	"github.com/moasq/nanowave/internal/revenuecatserver"
)

// registerIntegrationTools registers Supabase and RevenueCat CLI tools
// for non-MCP runtimes (Codex, OpenCode). These tools read credentials
// from the integration store and delegate to the same server client code.
func registerIntegrationTools(r *Registry) {
	// Supabase tools
	r.Register(supabaseExecuteSQLTool())
	r.Register(supabaseListTablesTool())
	r.Register(supabaseApplyMigrationTool())
	r.Register(supabaseGetProjectURLTool())
	r.Register(supabaseGetAnonKeyTool())
	r.Register(supabaseConfigureAuthTool())
	r.Register(supabaseGetAuthConfigTool())

	// RevenueCat tools
	r.Register(revenuecatListProductsTool())
	r.Register(revenuecatCreateProductTool())
	r.Register(revenuecatListEntitlementsTool())
	r.Register(revenuecatCreateEntitlementTool())
	r.Register(revenuecatGetAPIKeysTool())
	r.Register(revenuecatListAppsTool())
	r.Register(revenuecatListOfferingsTool())
	r.Register(revenuecatCreateOfferingTool())
}

// loadIntegrationConfig reads credentials for a provider from the integration store.
func loadIntegrationConfig(providerID, appName string) (*integrations.IntegrationConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot read home dir: %w", err)
	}
	store := integrations.NewIntegrationStore(filepath.Join(home, ".nanowave"))
	if err := store.Load(); err != nil {
		return nil, fmt.Errorf("failed to load integration store: %w", err)
	}

	reg := integrations.NewRegistry()
	providers.RegisterAll(reg)

	cfg, _ := store.GetProvider(integrations.ProviderID(providerID), appName)
	if cfg == nil {
		// Try without app name (global config)
		cfg, _ = store.GetProvider(integrations.ProviderID(providerID), "")
	}
	if cfg == nil {
		return nil, fmt.Errorf("%s is not configured. Run '/supabase' or '/revenuecat' to set up", providerID)
	}
	return cfg, nil
}

// setSupabaseEnv sets environment variables so supabaseserver client can authenticate.
func setSupabaseEnv(cfg *integrations.IntegrationConfig) (cleanup func()) {
	prevToken := os.Getenv("SUPABASE_ACCESS_TOKEN")
	prevRef := os.Getenv("SUPABASE_PROJECT_REF")
	os.Setenv("SUPABASE_ACCESS_TOKEN", cfg.PAT)
	os.Setenv("SUPABASE_PROJECT_REF", cfg.ProjectRef)
	return func() {
		if prevToken != "" {
			os.Setenv("SUPABASE_ACCESS_TOKEN", prevToken)
		} else {
			os.Unsetenv("SUPABASE_ACCESS_TOKEN")
		}
		if prevRef != "" {
			os.Setenv("SUPABASE_PROJECT_REF", prevRef)
		} else {
			os.Unsetenv("SUPABASE_PROJECT_REF")
		}
	}
}

// setRevenueCatEnv sets environment variables so revenuecatserver client can authenticate.
func setRevenueCatEnv(cfg *integrations.IntegrationConfig) (cleanup func()) {
	prevKey := os.Getenv("REVENUECAT_API_KEY")
	prevProject := os.Getenv("REVENUECAT_PROJECT_ID")
	os.Setenv("REVENUECAT_API_KEY", cfg.PAT)
	os.Setenv("REVENUECAT_PROJECT_ID", cfg.ProjectRef)
	return func() {
		if prevKey != "" {
			os.Setenv("REVENUECAT_API_KEY", prevKey)
		} else {
			os.Unsetenv("REVENUECAT_API_KEY")
		}
		if prevProject != "" {
			os.Setenv("REVENUECAT_PROJECT_ID", prevProject)
		} else {
			os.Unsetenv("REVENUECAT_PROJECT_ID")
		}
	}
}

// withSupabaseCredentials loads credentials and sets env vars for the duration of fn.
func withSupabaseCredentials(appName string, fn func() (json.RawMessage, error)) (json.RawMessage, error) {
	cfg, err := loadIntegrationConfig("supabase", appName)
	if err != nil {
		return jsonError(err.Error())
	}
	cleanup := setSupabaseEnv(cfg)
	defer cleanup()
	return fn()
}

// withRevenueCatCredentials loads credentials and sets env vars for the duration of fn.
func withRevenueCatCredentials(appName string, fn func() (json.RawMessage, error)) (json.RawMessage, error) {
	cfg, err := loadIntegrationConfig("revenuecat", appName)
	if err != nil {
		return jsonError(err.Error())
	}
	cleanup := setRevenueCatEnv(cfg)
	defer cleanup()
	return fn()
}

// ==================== Supabase Tools ====================

func supabaseExecuteSQLTool() *Tool {
	return &Tool{
		Name:        "nw_supabase_execute_sql",
		Description: "Execute a SQL query against the Supabase database. Returns rows as JSON.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "app_name": {"type": "string", "description": "App name for credential lookup"},
    "query": {"type": "string", "description": "SQL query to execute"}
  },
  "required": ["app_name", "query"]
}`),
		Handler: func(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				AppName string `json:"app_name"`
				Query   string `json:"query"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			return withSupabaseCredentials(in.AppName, func() (json.RawMessage, error) {
				result, err := supabaseserver.ExecuteSQL(ctx, in.Query)
				if err != nil {
					return jsonError(err.Error())
				}
				return jsonOK(json.RawMessage(result))
			})
		},
	}
}

func supabaseListTablesTool() *Tool {
	return &Tool{
		Name:        "nw_supabase_list_tables",
		Description: "List all tables in the Supabase database.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "app_name": {"type": "string", "description": "App name for credential lookup"},
    "schemas": {"type": "array", "items": {"type": "string"}, "description": "Schemas to list (default: public)"}
  },
  "required": ["app_name"]
}`),
		Handler: func(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				AppName string   `json:"app_name"`
				Schemas []string `json:"schemas"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			return withSupabaseCredentials(in.AppName, func() (json.RawMessage, error) {
				result, err := supabaseserver.ListTables(ctx, in.Schemas)
				if err != nil {
					return jsonError(err.Error())
				}
				return jsonOK(json.RawMessage(result))
			})
		},
	}
}

func supabaseApplyMigrationTool() *Tool {
	return &Tool{
		Name:        "nw_supabase_apply_migration",
		Description: "Apply a named database migration (CREATE TABLE, ALTER TABLE, etc.).",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "app_name": {"type": "string", "description": "App name for credential lookup"},
    "name": {"type": "string", "description": "Migration name e.g. create_users_table"},
    "statements": {"type": "array", "items": {"type": "string"}, "description": "SQL statements for the migration"}
  },
  "required": ["app_name", "name", "statements"]
}`),
		Handler: func(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				AppName    string   `json:"app_name"`
				Name       string   `json:"name"`
				Statements []string `json:"statements"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			return withSupabaseCredentials(in.AppName, func() (json.RawMessage, error) {
				if err := supabaseserver.ApplyMigration(ctx, in.Name, in.Statements); err != nil {
					return jsonError(err.Error())
				}
				return jsonOK(map[string]any{"success": true, "message": fmt.Sprintf("Migration %q applied", in.Name)})
			})
		},
	}
}

func supabaseGetProjectURLTool() *Tool {
	return &Tool{
		Name:        "nw_supabase_get_project_url",
		Description: "Get the Supabase project URL for Swift client configuration.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "app_name": {"type": "string", "description": "App name for credential lookup"}
  },
  "required": ["app_name"]
}`),
		Handler: func(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				AppName string `json:"app_name"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			cfg, err := loadIntegrationConfig("supabase", in.AppName)
			if err != nil {
				return jsonError(err.Error())
			}
			return jsonOK(map[string]string{"url": fmt.Sprintf("https://%s.supabase.co", cfg.ProjectRef)})
		},
	}
}

func supabaseGetAnonKeyTool() *Tool {
	return &Tool{
		Name:        "nw_supabase_get_anon_key",
		Description: "Get the Supabase anon (public) API key for Swift client configuration.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "app_name": {"type": "string", "description": "App name for credential lookup"}
  },
  "required": ["app_name"]
}`),
		Handler: func(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				AppName string `json:"app_name"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			cfg, err := loadIntegrationConfig("supabase", in.AppName)
			if err != nil {
				return jsonError(err.Error())
			}
			if cfg.AnonKey != "" {
				return jsonOK(map[string]string{"anon_key": cfg.AnonKey})
			}
			return withSupabaseCredentials(in.AppName, func() (json.RawMessage, error) {
				key, err := supabaseserver.GetAnonKey(ctx)
				if err != nil {
					return jsonError(err.Error())
				}
				return jsonOK(map[string]string{"anon_key": key})
			})
		},
	}
}

func supabaseConfigureAuthTool() *Tool {
	return &Tool{
		Name:        "nw_supabase_configure_auth",
		Description: "Enable or disable auth providers (apple, google, email, phone, anonymous) on the Supabase project.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "app_name": {"type": "string", "description": "App name for credential lookup"},
    "providers": {"type": "array", "items": {"type": "object", "properties": {"name": {"type": "string"}, "enabled": {"type": "boolean"}, "client_id": {"type": "string"}}}, "description": "Auth providers to configure"}
  },
  "required": ["app_name", "providers"]
}`),
		Handler: func(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				AppName   string `json:"app_name"`
				Providers []struct {
					Name     string `json:"name"`
					Enabled  bool   `json:"enabled"`
					ClientID string `json:"client_id"`
				} `json:"providers"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			return withSupabaseCredentials(in.AppName, func() (json.RawMessage, error) {
				var names []string
				for _, p := range in.Providers {
					names = append(names, p.Name)
				}
				// Delegate to the server's configure auth via env-based client
				result, err := supabaseserver.ConfigureAuth(ctx, input)
				if err != nil {
					return jsonError(err.Error())
				}
				return jsonOK(json.RawMessage(result))
			})
		},
	}
}

func supabaseGetAuthConfigTool() *Tool {
	return &Tool{
		Name:        "nw_supabase_get_auth_config",
		Description: "Get the current auth configuration for the Supabase project.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "app_name": {"type": "string", "description": "App name for credential lookup"}
  },
  "required": ["app_name"]
}`),
		Handler: func(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				AppName string `json:"app_name"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			return withSupabaseCredentials(in.AppName, func() (json.RawMessage, error) {
				result, err := supabaseserver.GetAuthConfig(ctx)
				if err != nil {
					return jsonError(err.Error())
				}
				return jsonOK(json.RawMessage(result))
			})
		},
	}
}

// ==================== RevenueCat Tools ====================

func revenuecatListProductsTool() *Tool {
	return &Tool{
		Name:        "nw_revenuecat_list_products",
		Description: "List all products in the RevenueCat project.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "app_name": {"type": "string", "description": "App name for credential lookup"}
  },
  "required": ["app_name"]
}`),
		Handler: func(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				AppName string `json:"app_name"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			return withRevenueCatCredentials(in.AppName, func() (json.RawMessage, error) {
				result, err := revenuecatserver.ListProducts(ctx)
				if err != nil {
					return jsonError(err.Error())
				}
				return jsonOK(json.RawMessage(result))
			})
		},
	}
}

func revenuecatCreateProductTool() *Tool {
	return &Tool{
		Name:        "nw_revenuecat_create_product",
		Description: "Create a new product in RevenueCat.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "app_name": {"type": "string", "description": "App name for credential lookup"},
    "store_identifier": {"type": "string", "description": "App Store product ID"},
    "app_id": {"type": "string", "description": "RevenueCat app ID"},
    "type": {"type": "string", "description": "Product type: subscription or one_time"}
  },
  "required": ["app_name", "store_identifier", "app_id", "type"]
}`),
		Handler: func(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				AppName         string `json:"app_name"`
				StoreIdentifier string `json:"store_identifier"`
				AppID           string `json:"app_id"`
				Type            string `json:"type"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			return withRevenueCatCredentials(in.AppName, func() (json.RawMessage, error) {
				result, err := revenuecatserver.CreateProduct(ctx, in.StoreIdentifier, in.AppID, in.Type)
				if err != nil {
					return jsonError(err.Error())
				}
				return jsonOK(json.RawMessage(result))
			})
		},
	}
}

func revenuecatListEntitlementsTool() *Tool {
	return &Tool{
		Name:        "nw_revenuecat_list_entitlements",
		Description: "List all entitlements in the RevenueCat project.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "app_name": {"type": "string", "description": "App name for credential lookup"}
  },
  "required": ["app_name"]
}`),
		Handler: func(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				AppName string `json:"app_name"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			return withRevenueCatCredentials(in.AppName, func() (json.RawMessage, error) {
				result, err := revenuecatserver.ListEntitlements(ctx)
				if err != nil {
					return jsonError(err.Error())
				}
				return jsonOK(json.RawMessage(result))
			})
		},
	}
}

func revenuecatCreateEntitlementTool() *Tool {
	return &Tool{
		Name:        "nw_revenuecat_create_entitlement",
		Description: "Create a new entitlement in RevenueCat (e.g. 'premium' access level).",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "app_name": {"type": "string", "description": "App name for credential lookup"},
    "lookup_key": {"type": "string", "description": "Entitlement lookup key e.g. premium"},
    "display_name": {"type": "string", "description": "Display name"}
  },
  "required": ["app_name", "lookup_key", "display_name"]
}`),
		Handler: func(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				AppName     string `json:"app_name"`
				LookupKey   string `json:"lookup_key"`
				DisplayName string `json:"display_name"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			return withRevenueCatCredentials(in.AppName, func() (json.RawMessage, error) {
				result, err := revenuecatserver.CreateEntitlement(ctx, in.LookupKey, in.DisplayName)
				if err != nil {
					return jsonError(err.Error())
				}
				return jsonOK(json.RawMessage(result))
			})
		},
	}
}

func revenuecatGetAPIKeysTool() *Tool {
	return &Tool{
		Name:        "nw_revenuecat_get_api_keys",
		Description: "Get the public SDK API keys for initializing the RevenueCat SDK in Swift.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "app_name": {"type": "string", "description": "App name for credential lookup"},
    "rc_app_id": {"type": "string", "description": "RevenueCat app ID"}
  },
  "required": ["app_name", "rc_app_id"]
}`),
		Handler: func(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				AppName string `json:"app_name"`
				RCAppID string `json:"rc_app_id"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			return withRevenueCatCredentials(in.AppName, func() (json.RawMessage, error) {
				result, err := revenuecatserver.GetPublicAPIKeys(ctx, in.RCAppID)
				if err != nil {
					return jsonError(err.Error())
				}
				return jsonOK(json.RawMessage(result))
			})
		},
	}
}

func revenuecatListAppsTool() *Tool {
	return &Tool{
		Name:        "nw_revenuecat_list_apps",
		Description: "List all apps in the RevenueCat project.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "app_name": {"type": "string", "description": "App name for credential lookup"}
  },
  "required": ["app_name"]
}`),
		Handler: func(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				AppName string `json:"app_name"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			return withRevenueCatCredentials(in.AppName, func() (json.RawMessage, error) {
				result, err := revenuecatserver.ListApps(ctx)
				if err != nil {
					return jsonError(err.Error())
				}
				return jsonOK(json.RawMessage(result))
			})
		},
	}
}

func revenuecatListOfferingsTool() *Tool {
	return &Tool{
		Name:        "nw_revenuecat_list_offerings",
		Description: "List all offerings in the RevenueCat project.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "app_name": {"type": "string", "description": "App name for credential lookup"}
  },
  "required": ["app_name"]
}`),
		Handler: func(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				AppName string `json:"app_name"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			return withRevenueCatCredentials(in.AppName, func() (json.RawMessage, error) {
				result, err := revenuecatserver.ListOfferings(ctx)
				if err != nil {
					return jsonError(err.Error())
				}
				return jsonOK(json.RawMessage(result))
			})
		},
	}
}

func revenuecatCreateOfferingTool() *Tool {
	return &Tool{
		Name:        "nw_revenuecat_create_offering",
		Description: "Create a new offering in RevenueCat.",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "app_name": {"type": "string", "description": "App name for credential lookup"},
    "lookup_key": {"type": "string", "description": "Offering lookup key e.g. default"},
    "display_name": {"type": "string", "description": "Display name"},
    "is_current": {"type": "boolean", "description": "Whether this is the current (default) offering", "default": false}
  },
  "required": ["app_name", "lookup_key", "display_name"]
}`),
		Handler: func(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
			var in struct {
				AppName     string `json:"app_name"`
				LookupKey   string `json:"lookup_key"`
				DisplayName string `json:"display_name"`
				IsCurrent   bool   `json:"is_current"`
			}
			if err := json.Unmarshal(input, &in); err != nil {
				return jsonError("invalid input: " + err.Error())
			}
			return withRevenueCatCredentials(in.AppName, func() (json.RawMessage, error) {
				result, err := revenuecatserver.CreateOffering(ctx, in.LookupKey, in.DisplayName, in.IsCurrent)
				if err != nil {
					return jsonError(err.Error())
				}
				return jsonOK(json.RawMessage(result))
			})
		},
	}
}

// Suppress unused import warnings
var _ = strings.TrimSpace
