# API Keys Retrieval

## Contents
- Retrieving project API keys via Management API
- Key types and identification
- Using keys in Swift client configuration

## Endpoint

```
GET /v1/projects/{ref}/api-keys?reveal=true
Authorization: Bearer <PAT>
```

OAuth scope: `secrets:read`

### Path Parameters

| Parameter | Type | Constraints |
|-----------|------|-------------|
| `ref` | string | Exactly 20 lowercase alpha characters |

### Query Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `reveal` | boolean | When `true`, returns full key values. Secret keys are hidden otherwise. |

## Response (200 OK)

JSON array of key objects:

```json
[
  {
    "name": "anon",
    "api_key": "eyJhbGciOiJIUzI1NiIs...",
    "id": "uuid",
    "type": "legacy",
    "prefix": null,
    "description": "This key is safe to use in a browser...",
    "hash": "...",
    "inserted_at": "2025-01-15T...",
    "updated_at": "2025-01-15T..."
  },
  {
    "name": "service_role",
    "api_key": "eyJhbGciOiJIUzI1NiIs...",
    "id": "uuid",
    "type": "legacy",
    "prefix": null,
    "description": "This key has the ability to bypass Row Level Security...",
    "hash": "...",
    "inserted_at": "2025-01-15T...",
    "updated_at": "2025-01-15T..."
  }
]
```

## Key Types

### Legacy Keys (Current)

| Name | Role | Client-Safe |
|------|------|-------------|
| `anon` | Respects RLS, safe for client-side | Yes |
| `service_role` | Bypasses RLS, admin access | No |

### New Keys (Migration in Progress)

| Type | Prefix | Replaces |
|------|--------|----------|
| `publishable` | `sb_publishable_` | `anon` |
| `secret` | `sb_secret_` | `service_role` |

## Finding the Anon Key

Filter by `name == "anon"` (case-insensitive) for legacy keys, or `type == "publishable"` for new keys:

```go
for _, key := range keys {
    if strings.EqualFold(key.Name, "anon") || key.Type == "publishable" {
        return key.APIKey
    }
}
```

## Pipeline Usage

The pipeline retrieves the anon key to inject into the generated `AppConfig.swift`:

```swift
enum AppConfig {
    static let supabaseURL = "https://PROJECT_REF.supabase.co"
    static let supabaseAnonKey = "RETRIEVED_ANON_KEY"
}
```

This allows the app to connect to Supabase without manual credential configuration.

## Error Responses

| Status | Description |
|--------|-------------|
| 401 | Unauthorized (invalid or missing Bearer token) |
| 403 | Forbidden (insufficient permissions) |
| 429 | Rate limit exceeded |
