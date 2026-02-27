# Secrets API

## Contents
- Managing edge function environment variables via Management API
- Creating, listing, and deleting secrets
- Name and value constraints

## Endpoint Reference

Base URL: `https://api.supabase.com/v1/projects/{ref}/secrets`

Auth: `Authorization: Bearer <PAT>` (OAuth scope: `secrets:write` / `secrets:read`)

### Create/Update Secrets (Bulk)

```
POST /v1/projects/{ref}/secrets
Content-Type: application/json

[
  { "name": "MY_API_KEY", "value": "sk-abc123..." },
  { "name": "WEBHOOK_URL", "value": "https://example.com/hook" }
]
```

- Body is a JSON **array** of `{ name, value }` objects
- Name: max 256 chars, must NOT start with `SUPABASE_`
- Value: max 24,576 chars
- Existing secrets with the same name are overwritten (upsert)

### List Secrets

```
GET /v1/projects/{ref}/secrets
```

Response:
```json
[
  { "name": "MY_API_KEY", "value": "sk-abc123...", "updated_at": "2025-01-15T..." }
]
```

### Delete Secrets (Bulk)

```
DELETE /v1/projects/{ref}/secrets
Content-Type: application/json

["MY_API_KEY", "WEBHOOK_URL"]
```

Body is a JSON **array of name strings**.

## Usage in Pipeline

Secrets are injected as environment variables into all Edge Functions in the project.

Common use cases:
- Third-party API keys (Stripe, SendGrid, OpenAI)
- Webhook signing secrets
- Custom configuration values

```sql
-- Edge functions access secrets via Deno.env
-- Example in Edge Function TypeScript:
-- const apiKey = Deno.env.get('MY_API_KEY')
```

## Constraints

| Field | Limit |
|-------|-------|
| Name max length | 256 characters |
| Name prefix | Must NOT start with `SUPABASE_` |
| Value max length | 24,576 characters |
| Operations | Bulk (array-based) for create and delete |
