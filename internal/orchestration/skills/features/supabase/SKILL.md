---
name: "supabase"
description: "Supabase Swift SDK patterns for auth, database, and storage. Use when implementing app features related to supabase."
---
# Supabase Integration

## CRITICAL: Backend-First Execution Order

You MUST set up the Supabase backend BEFORE writing any Swift code. Follow this exact order:

### Phase 1: Backend Setup (via Supabase MCP)
1. **Create all tables** with `mcp__supabase__execute_sql` — columns, types, foreign keys, constraints, indexes
2. **Enable RLS** on every table — `ALTER TABLE ... ENABLE ROW LEVEL SECURITY`
3. **Create RLS policies** for every table — SELECT, INSERT, UPDATE, DELETE as appropriate
4. **Create storage buckets** — public for images, private for documents
5. **Create storage policies** — per-user folder enforcement on `storage.objects`
6. **Create triggers** — for `updated_at`, denormalized counters, etc.
7. **Verify** with `mcp__supabase__list_tables` and `mcp__supabase__list_storage_buckets`
8. **Check auth config** with `mcp__supabase__get_auth_config` to confirm providers are enabled (auto-configured by nanowave)

### Phase 2: Swift Code
8. Write `Config/AppConfig.swift` — Supabase URL + anon key
9. Write `SupabaseService.swift` — shared client
10. Write models with `Codable` + `CodingKeys` matching the table columns you just created
11. Write services that query the tables you just created
12. Write views

**NEVER skip Phase 1.** If you write Swift code that references tables that don't exist, the app will crash at runtime.

## Client Initialization

Initialize `SupabaseClient` once in a shared service. Use PKCE auth flow (default for mobile).

```swift
import Supabase

@Observable
final class SupabaseService {
    static let shared = SupabaseService()
    let client: SupabaseClient

    private init() {
        client = SupabaseClient(
            supabaseURL: URL(string: AppConfig.supabaseURL)!,
            supabaseKey: AppConfig.supabaseAnonKey
        )
    }
}
```

## AppConfig Pattern

Store Supabase credentials as static constants — injected by nanowave during build.

```swift
enum AppConfig {
    static let supabaseURL = "https://PROJECT_REF.supabase.co"
    static let supabaseAnonKey = "YOUR_ANON_KEY"
}
```

## Key Rules

- **Backend first** — create tables, RLS, buckets via MCP before writing Swift code
- **Never manage tokens manually** — Supabase SDK auto-refreshes sessions
- **Auth architecture** is handled by the `authentication` skill (AuthService, guards, modes) — this skill covers only the Supabase auth API calls
- **Data access architecture** (repository protocols, DTOs, domain mapping) is handled by the `repositories` skill — this skill covers Supabase API patterns used inside concrete repository implementations
- **Models use Codable** (NOT @Model) — Supabase is the persistence layer, not SwiftData
- **All operations are async/await** — no callbacks, no Combine
- **RLS on every table** — never leave a table without Row Level Security
- **Use XcodeGen MCP** to add "Sign in with Apple" entitlement when auth is needed
- **Auth providers are auto-configured** by the nanowave pipeline — use `mcp__supabase__get_auth_config` to verify, use `mcp__supabase__configure_auth_providers` only if manual adjustment is needed

## Available MCP Tools

- `mcp__supabase__execute_sql` — run SQL queries (SELECT, DML)
- `mcp__supabase__list_tables` — list tables in schemas
- `mcp__supabase__apply_migration` — track DDL as versioned migrations
- `mcp__supabase__list_storage_buckets` — list storage buckets
- `mcp__supabase__get_project_url` — get project URL for Swift client
- `mcp__supabase__get_anon_key` — get anon key for Swift client
- `mcp__supabase__get_logs` — query project logs
- `mcp__supabase__configure_auth_providers` — enable/disable auth providers (apple, google, email, phone, anonymous)
- `mcp__supabase__get_auth_config` — check current auth provider configuration
- `mcp__supabase__set_secrets` — set edge function environment variables (name/value pairs)
- `mcp__supabase__list_secrets` — list all project secrets
- `mcp__supabase__delete_secrets` — delete secrets by name

## References

### Core
- [Schema Setup](references/schema-setup.md) — table creation, types, foreign keys, triggers
- [RLS Policies](references/rls-policies.md) — Row Level Security patterns for every table type
- [Auth Patterns](references/auth-patterns.md) — email auth, Apple Sign In, auth state, guards
- [Database Patterns](references/database-patterns.md) — CRUD, filtering, realtime subscriptions

### Storage
- [Storage Setup](references/storage-setup.md) — bucket creation, storage policies, path conventions
- [Storage Patterns](references/storage-patterns.md) — upload, download, public/signed URLs
- [Storage Service](references/storage-service.md) — StorageService singleton, image compression, PhotosPicker flow, ViewModel upload patterns

### Management API
- [Secrets API](references/secrets-api.md) — edge function environment variables (create, list, delete)
- [Edge Functions](references/edge-functions.md) — deploying and managing edge functions via API
- [API Keys](references/api-keys.md) — retrieving project anon/service_role keys
- [Realtime](references/realtime.md) — enabling per-table realtime via SQL publication
- [Webhooks & Triggers](references/webhooks-triggers.md) — database webhooks, pg_net, Vault integration
