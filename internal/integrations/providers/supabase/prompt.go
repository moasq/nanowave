package supabase

import (
	"context"
	"fmt"
	"strings"

	"github.com/moasq/nanowave/internal/integrations"
)

// PromptContribution generates the Supabase integration prompt content.
// This replicates the logic from orchestration/build_prompts.go appendIntegrationConfig().
func (s *supabaseProvider) PromptContribution(_ context.Context, req integrations.PromptRequest) (*integrations.PromptContribution, error) {
	cfg, _ := req.Store.GetProvider(integrations.ProviderSupabase, req.AppName)

	var system strings.Builder
	system.WriteString("\n<integration-config>\n")

	// Credentials
	if cfg != nil && cfg.ProjectURL != "" {
		fmt.Fprintf(&system, "Supabase Project URL: %s\n", cfg.ProjectURL)
		fmt.Fprintf(&system, "Supabase Anon Key: %s\n", cfg.AnonKey)
		system.WriteString("Store these in Config/AppConfig.swift as static constants.\n\n")
	} else {
		system.WriteString("Supabase Project URL: https://YOUR_PROJECT_REF.supabase.co\n")
		system.WriteString("Supabase Anon Key: YOUR_ANON_KEY\n")
		system.WriteString("Store these in Config/AppConfig.swift as static constants. The user will replace the placeholders.\n\n")
	}

	// Backend setup instructions (only if MCP available via PAT)
	hasMCP := cfg != nil && cfg.PAT != ""
	if hasMCP {
		// Auth provider status
		if len(req.AuthMethods) > 0 {
			system.WriteString("## Auth Providers (auto-configured by nanowave)\n\n")
			fmt.Fprintf(&system, "Auth providers already configured: %s.\n", strings.Join(req.AuthMethods, ", "))
			system.WriteString("Do NOT configure auth providers manually — they are already enabled on the Supabase project.\n\n")
		}

		system.WriteString("<backend-setup>\n")
		system.WriteString("## MANDATORY: Backend-First Execution Order\n\n")
		system.WriteString("The Supabase MCP server is connected. You MUST set up the backend BEFORE writing any Swift code.\n\n")
		system.WriteString("### Step 1: Create ALL tables (use mcp__supabase__execute_sql)\n")
		system.WriteString("Create every table needed by the app's models. Include columns, types, foreign keys, constraints, and indexes.\n")
		system.WriteString("Use IF NOT EXISTS for idempotency. Always use snake_case column names.\n\n")

		// Model → table mapping
		if len(req.Models) > 0 {
			system.WriteString("### Required Tables (derived from planned models)\n\n")
			for _, m := range req.Models {
				tableName := integrations.ModelRefTableName(m.Name)
				fmt.Fprintf(&system, "**Table: `%s`** (from model `%s`)\n", tableName, m.Name)
				system.WriteString("```sql\nCREATE TABLE IF NOT EXISTS public." + tableName + " (\n")
				for i, prop := range m.Properties {
					colName := modelRefCamelToSnake(prop.Name)
					pgType := swiftTypeToPG(prop.Type)
					constraints := inferConstraints(prop, i == 0, m.Name)
					fmt.Fprintf(&system, "  %s %s%s", colName, pgType, constraints)
					if i < len(m.Properties)-1 {
						system.WriteString(",")
					}
					system.WriteString("\n")
				}
				system.WriteString(");\n```\n\n")
			}
		}

		system.WriteString("### Step 2: Enable RLS on every table\n")
		system.WriteString("```sql\n")
		for _, m := range req.Models {
			fmt.Fprintf(&system, "ALTER TABLE public.%s ENABLE ROW LEVEL SECURITY;\n", integrations.ModelRefTableName(m.Name))
		}
		system.WriteString("```\n\n")

		system.WriteString("### Step 3: Create RLS policies for every table\n")
		system.WriteString("See the supabase skill's RLS reference for patterns: public-read + owner-write for content tables, ")
		system.WriteString("actor-write for join tables, owner-only for private data.\n\n")

		system.WriteString("### Step 4: Create storage buckets and policies\n")
		system.WriteString("If the app uploads files (images, documents), create the storage bucket and policies.\n")
		system.WriteString("See the supabase skill's storage-setup reference for patterns.\n\n")

		system.WriteString("### Step 5: Verify\n")
		system.WriteString("Use mcp__supabase__list_tables to confirm all tables exist before proceeding.\n\n")

		system.WriteString("### Step 6: STOP and verify before writing Swift code\n")
		system.WriteString("Call `mcp__supabase__list_tables` NOW. Only proceed to Swift code after confirming tables exist.\n\n")
		system.WriteString("</backend-setup>\n\n")
	}

	system.WriteString("Models use Codable (NOT @Model) — Supabase is the persistence layer.\n")
	system.WriteString("</integration-config>\n")

	// User message block (backend-first instructions)
	var userBlock string
	if hasMCP {
		if req.BackendProvisioned {
			userBlock = `
SUPABASE BACKEND (already provisioned by nanowave):
Tables, RLS policies, and storage buckets have been created automatically.
1. Use mcp__supabase__list_tables to see the available tables.
2. If you need additional tables, indexes, or policies, use mcp__supabase__execute_sql.
3. Proceed directly to writing Swift code — the backend is ready.

`
		} else {
			userBlock = `
CRITICAL — BACKEND FIRST (before writing ANY Swift code):
1. Read the <backend-setup> section in the system prompt — it has the exact SQL.
2. Use mcp__supabase__execute_sql to create ALL tables defined there (run every CREATE TABLE statement).
3. Use mcp__supabase__execute_sql to enable RLS on every table and create RLS policies.
4. If the app has file uploads, create storage buckets and policies.
5. Use mcp__supabase__list_tables to VERIFY all tables exist.
6. Only after tables are confirmed — proceed to write Swift code.
DO NOT skip this. The app CANNOT function without a backend schema.

`
		}
	}

	return &integrations.PromptContribution{
		SystemBlock:        system.String(),
		UserBlock:          userBlock,
		BackendProvisioned: req.BackendProvisioned,
	}, nil
}

// --- Helper functions (mirrored from orchestration/build_prompts.go) ---

// modelRefCamelToSnake converts camelCase/PascalCase to snake_case.
func modelRefCamelToSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r + 32) // toLower
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// swiftTypeToPG maps Swift type strings to PostgreSQL types.
func swiftTypeToPG(swiftType string) string {
	t := strings.TrimSuffix(swiftType, "?")
	switch t {
	case "UUID":
		return "UUID"
	case "String":
		return "TEXT"
	case "Int":
		return "INTEGER"
	case "Double", "Float":
		return "DOUBLE PRECISION"
	case "Bool":
		return "BOOLEAN"
	case "Date":
		return "TIMESTAMPTZ"
	case "URL":
		return "TEXT"
	case "[String]":
		return "TEXT[]"
	default:
		if len(t) > 0 && t[0] >= 'A' && t[0] <= 'Z' {
			return "UUID"
		}
		return "TEXT"
	}
}

// inferConstraints generates SQL constraints for a property.
func inferConstraints(prop integrations.PropertyRef, isFirst bool, _ string) string {
	var parts []string
	isOptional := strings.HasSuffix(prop.Type, "?")

	if isFirst && strings.ToLower(prop.Name) == "id" {
		parts = append(parts, "PRIMARY KEY")
		if prop.Type == "UUID" {
			parts = append(parts, "DEFAULT gen_random_uuid()")
		}
	}

	if !isOptional && !isFirst {
		parts = append(parts, "NOT NULL")
	}

	if prop.DefaultValue != "" {
		parts = append(parts, fmt.Sprintf("DEFAULT %s", prop.DefaultValue))
	}

	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
}
