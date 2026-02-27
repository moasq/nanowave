package supabaseserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type textOutput struct {
	Message string `json:"message"`
}

// --- execute_sql ---

type executeSQLInput struct {
	Query string `json:"query" jsonschema:"The SQL query to execute"`
}

func handleExecuteSQL(ctx context.Context, req *mcp.CallToolRequest, input executeSQLInput) (*mcp.CallToolResult, textOutput, error) {
	if input.Query == "" {
		return nil, textOutput{}, fmt.Errorf("query is required")
	}
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	raw, err := c.executeSQL(ctx, input.Query)
	if err != nil {
		return nil, textOutput{}, err
	}
	return nil, textOutput{Message: string(raw)}, nil
}

// --- list_tables ---

type listTablesInput struct {
	Schemas []string `json:"schemas" jsonschema:"Schemas to list tables from (default: public)"`
}

func handleListTables(ctx context.Context, req *mcp.CallToolRequest, input listTablesInput) (*mcp.CallToolResult, textOutput, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	raw, err := c.listTables(ctx, input.Schemas)
	if err != nil {
		return nil, textOutput{}, err
	}
	return nil, textOutput{Message: string(raw)}, nil
}

// --- apply_migration ---

type applyMigrationInput struct {
	Name       string   `json:"name" jsonschema:"Migration name (e.g. create_users_table)"`
	Statements []string `json:"statements" jsonschema:"SQL statements for this migration"`
}

func handleApplyMigration(ctx context.Context, req *mcp.CallToolRequest, input applyMigrationInput) (*mcp.CallToolResult, textOutput, error) {
	if input.Name == "" {
		return nil, textOutput{}, fmt.Errorf("name is required")
	}
	if len(input.Statements) == 0 {
		return nil, textOutput{}, fmt.Errorf("at least one statement is required")
	}
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	if err := c.applyMigration(ctx, input.Name, input.Statements); err != nil {
		return nil, textOutput{}, err
	}
	return nil, textOutput{Message: fmt.Sprintf("Migration %q applied successfully.", input.Name)}, nil
}

// --- list_storage_buckets ---

type listStorageBucketsInput struct{}

func handleListStorageBuckets(ctx context.Context, req *mcp.CallToolRequest, input listStorageBucketsInput) (*mcp.CallToolResult, textOutput, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	raw, err := c.listStorageBuckets(ctx)
	if err != nil {
		return nil, textOutput{}, err
	}
	return nil, textOutput{Message: string(raw)}, nil
}

// --- get_project_url ---

type getProjectURLInput struct{}

func handleGetProjectURL(ctx context.Context, req *mcp.CallToolRequest, input getProjectURLInput) (*mcp.CallToolResult, textOutput, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	return nil, textOutput{Message: fmt.Sprintf("https://%s.supabase.co", c.projectRef)}, nil
}

// --- get_anon_key ---

type getAnonKeyInput struct{}

func handleGetAnonKey(ctx context.Context, req *mcp.CallToolRequest, input getAnonKeyInput) (*mcp.CallToolResult, textOutput, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	keys, err := c.getAPIKeys(ctx)
	if err != nil {
		return nil, textOutput{}, err
	}
	for _, k := range keys {
		if k.Name == "anon" || strings.Contains(k.Name, "anon") {
			return nil, textOutput{Message: k.APIKey}, nil
		}
	}
	return nil, textOutput{}, fmt.Errorf("anon key not found in project API keys")
}

// --- get_logs ---

type getLogsInput struct {
	SQL string `json:"sql" jsonschema:"SQL query to filter logs"`
}

func handleGetLogs(ctx context.Context, req *mcp.CallToolRequest, input getLogsInput) (*mcp.CallToolResult, textOutput, error) {
	if input.SQL == "" {
		return nil, textOutput{}, fmt.Errorf("sql query is required")
	}
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	raw, err := c.getLogs(ctx, input.SQL)
	if err != nil {
		return nil, textOutput{}, err
	}
	return nil, textOutput{Message: string(raw)}, nil
}

// --- configure_auth_providers ---

type configureAuthInput struct {
	Providers []authProviderConfig `json:"providers" jsonschema:"Auth providers to configure"`
}

type authProviderConfig struct {
	Name     string `json:"name" jsonschema:"Provider name: apple, google, email, phone"`
	Enabled  bool   `json:"enabled" jsonschema:"Whether to enable this provider"`
	ClientID string `json:"client_id" jsonschema:"Client ID (for Apple: bundle ID; for Google: OAuth client ID)"`
	Secret   string `json:"secret" jsonschema:"Client secret (optional, not needed for native Apple Sign In)"`
}

// providerConfigMap maps a provider config to the Supabase Management API auth config fields.
var providerConfigMap = map[string]func(p authProviderConfig) map[string]any{
	"apple": func(p authProviderConfig) map[string]any {
		m := map[string]any{"EXTERNAL_APPLE_ENABLED": p.Enabled}
		if p.ClientID != "" {
			m["EXTERNAL_APPLE_CLIENT_IDS"] = []map[string]string{{"client_id": p.ClientID}}
		}
		if p.Secret != "" {
			m["EXTERNAL_APPLE_SECRET"] = p.Secret
		}
		return m
	},
	"google": func(p authProviderConfig) map[string]any {
		m := map[string]any{"EXTERNAL_GOOGLE_ENABLED": p.Enabled}
		if p.ClientID != "" {
			m["EXTERNAL_GOOGLE_CLIENT_ID"] = p.ClientID
		}
		if p.Secret != "" {
			m["EXTERNAL_GOOGLE_SECRET"] = p.Secret
		}
		return m
	},
	"phone": func(p authProviderConfig) map[string]any {
		return map[string]any{"EXTERNAL_PHONE_ENABLED": p.Enabled}
	},
}

func handleConfigureAuthProviders(ctx context.Context, req *mcp.CallToolRequest, input configureAuthInput) (*mcp.CallToolResult, textOutput, error) {
	if len(input.Providers) == 0 {
		return nil, textOutput{}, fmt.Errorf("at least one provider is required")
	}
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}

	config := make(map[string]any)
	var configured []string

	for _, p := range input.Providers {
		switch p.Name {
		case "email":
			// Email is enabled by default on Supabase — no action needed
			configured = append(configured, "email (enabled by default)")
			continue
		case "anonymous":
			config["EXTERNAL_ANONYMOUS_USERS_ENABLED"] = p.Enabled
			configured = append(configured, "anonymous")
			continue
		}

		mapper, ok := providerConfigMap[p.Name]
		if !ok {
			return nil, textOutput{}, fmt.Errorf("unsupported provider: %s (supported: apple, google, email, phone, anonymous)", p.Name)
		}
		for k, v := range mapper(p) {
			config[k] = v
		}
		configured = append(configured, p.Name)
	}

	if len(config) > 0 {
		if err := c.updateAuthConfig(ctx, config); err != nil {
			return nil, textOutput{}, fmt.Errorf("update auth config: %w", err)
		}
	}

	result, _ := json.Marshal(map[string]any{
		"configured_providers": configured,
		"status":               "success",
	})
	return nil, textOutput{Message: string(result)}, nil
}

// --- get_auth_config ---

type getAuthConfigInput struct{}

func handleGetAuthConfig(ctx context.Context, req *mcp.CallToolRequest, input getAuthConfigInput) (*mcp.CallToolResult, textOutput, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	raw, err := c.getAuthConfig(ctx)
	if err != nil {
		return nil, textOutput{}, err
	}
	return nil, textOutput{Message: string(raw)}, nil
}

// --- set_secrets ---

type setSecretsInput struct {
	Secrets []secretEntry `json:"secrets" jsonschema:"Array of secrets to set"`
}

type secretEntry struct {
	Name  string `json:"name" jsonschema:"Secret name (max 256 chars, must NOT start with SUPABASE_)"`
	Value string `json:"value" jsonschema:"Secret value (max 24576 chars)"`
}

func handleSetSecrets(ctx context.Context, req *mcp.CallToolRequest, input setSecretsInput) (*mcp.CallToolResult, textOutput, error) {
	if len(input.Secrets) == 0 {
		return nil, textOutput{}, fmt.Errorf("at least one secret is required")
	}
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	secrets := make([]map[string]string, len(input.Secrets))
	for i, s := range input.Secrets {
		secrets[i] = map[string]string{"name": s.Name, "value": s.Value}
	}
	if err := c.setSecrets(ctx, secrets); err != nil {
		return nil, textOutput{}, err
	}
	names := make([]string, len(input.Secrets))
	for i, s := range input.Secrets {
		names[i] = s.Name
	}
	return nil, textOutput{Message: fmt.Sprintf("Secrets set: %s", strings.Join(names, ", "))}, nil
}

// --- list_secrets ---

type listSecretsInput struct{}

func handleListSecrets(ctx context.Context, req *mcp.CallToolRequest, input listSecretsInput) (*mcp.CallToolResult, textOutput, error) {
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	raw, err := c.listSecrets(ctx)
	if err != nil {
		return nil, textOutput{}, err
	}
	return nil, textOutput{Message: string(raw)}, nil
}

// --- delete_secrets ---

type deleteSecretsInput struct {
	Names []string `json:"names" jsonschema:"Secret names to delete"`
}

func handleDeleteSecrets(ctx context.Context, req *mcp.CallToolRequest, input deleteSecretsInput) (*mcp.CallToolResult, textOutput, error) {
	if len(input.Names) == 0 {
		return nil, textOutput{}, fmt.Errorf("at least one secret name is required")
	}
	c, err := newClientFromEnv()
	if err != nil {
		return nil, textOutput{}, err
	}
	if err := c.deleteSecrets(ctx, input.Names); err != nil {
		return nil, textOutput{}, err
	}
	return nil, textOutput{Message: fmt.Sprintf("Secrets deleted: %s", strings.Join(input.Names, ", "))}, nil
}
