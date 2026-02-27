package supabaseserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Run starts the Supabase MCP server over stdio.
// It blocks until the client disconnects or the context is cancelled.
func Run(ctx context.Context) error {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "supabase",
			Version: "v1.0.0",
		},
		nil,
	)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "execute_sql",
		Description: "Execute a SQL query against the Supabase database. Returns rows as JSON. Use for SELECT queries and DML (INSERT, UPDATE, DELETE). For DDL/schema changes, prefer apply_migration.",
	}, handleExecuteSQL)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_tables",
		Description: "List all tables in the specified schemas (default: public). Returns table_schema, table_name, and table_type from information_schema.",
	}, handleListTables)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "apply_migration",
		Description: "Apply a named database migration. Tracks DDL changes (CREATE TABLE, ALTER TABLE, etc.) as versioned migrations. Use for all schema changes.",
	}, handleApplyMigration)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_storage_buckets",
		Description: "List all storage buckets in the Supabase project. Returns bucket ID, name, public status, and size limits.",
	}, handleListStorageBuckets)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_project_url",
		Description: "Get the Supabase project URL (https://<ref>.supabase.co). Use this to configure the Swift client.",
	}, handleGetProjectURL)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_anon_key",
		Description: "Get the project's anon (public) API key. Use this to configure the Swift client.",
	}, handleGetAnonKey)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_logs",
		Description: "Query recent project logs. Provide a SQL query to filter logs from the analytics endpoint.",
	}, handleGetLogs)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "configure_auth_providers",
		Description: "Enable or disable auth providers on the Supabase project. Supports apple, google, email, phone. For Apple Sign In on iOS, only the bundle ID is needed as client_id (no secret required for native signInWithIdToken).",
	}, handleConfigureAuthProviders)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_auth_config",
		Description: "Get the current auth configuration for the project. Returns which providers are enabled and their settings.",
	}, handleGetAuthConfig)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "set_secrets",
		Description: "Set edge function secrets (environment variables). Secrets are available to all edge functions via Deno.env.get(). Names must not start with SUPABASE_. Existing secrets with the same name are overwritten.",
	}, handleSetSecrets)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_secrets",
		Description: "List all project secrets (edge function environment variables). Returns name, value, and updated_at for each secret.",
	}, handleListSecrets)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_secrets",
		Description: "Delete edge function secrets by name.",
	}, handleDeleteSecrets)

	return server.Run(ctx, &mcp.StdioTransport{})
}
