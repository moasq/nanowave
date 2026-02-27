# Edge Functions

## Contents
- Deploying Edge Functions via Management API
- Function metadata and file upload
- Listing, updating, and deleting functions
- Calling Edge Functions from triggers

## Deploy Endpoint (Recommended)

```
POST /v1/projects/{ref}/functions/deploy?slug=my-function
Content-Type: multipart/form-data
Authorization: Bearer <PAT>
```

### Multipart Form Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `metadata` | JSON string | Yes | Function configuration |
| `file` | binary | No | Source files (repeat for multiple files) |

### Metadata Object

```json
{
  "entrypoint_path": "index.ts",
  "name": "my-function",
  "verify_jwt": true,
  "import_map_path": "import_map.json",
  "static_patterns": ["assets/**"]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `entrypoint_path` | string | Yes | Relative path to entry point |
| `name` | string | No | Display name |
| `verify_jwt` | boolean | No | Require JWT verification (default: true) |
| `import_map_path` | string | No | Deno import map path |
| `static_patterns` | string[] | No | Glob patterns for static files |

### Response (201 Created)

```json
{
  "id": "uuid",
  "slug": "my-function",
  "name": "my-function",
  "status": "ACTIVE",
  "version": 1,
  "created_at": 1234567890,
  "updated_at": 1234567890,
  "verify_jwt": true,
  "import_map": true,
  "entrypoint_path": "index.ts"
}
```

Status enum: `ACTIVE`, `REMOVED`, `THROTTLED`

### Slug Constraints

Pattern: `^[A-Za-z][A-Za-z0-9_-]*$` — must start with a letter.

## Other Endpoints

### List Functions

```
GET /v1/projects/{ref}/functions
```

Returns array of function objects.

### Get Function

```
GET /v1/projects/{ref}/functions/{function_slug}
```

### Update Function

```
PATCH /v1/projects/{ref}/functions/{function_slug}
```

Accepts `multipart/form-data` or `application/json` with optional `name`, `body`, `verify_jwt`.

### Delete Function

```
DELETE /v1/projects/{ref}/functions/{function_slug}
```

## Calling Edge Functions from Database Triggers

Use `pg_net` for async HTTP calls from triggers:

```sql
CREATE OR REPLACE FUNCTION public.on_new_order()
RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER
SET search_path = public AS $$
BEGIN
  PERFORM net.http_post(
    url := 'https://PROJECT_REF.supabase.co/functions/v1/on-new-order',
    headers := jsonb_build_object(
      'Content-Type', 'application/json',
      'Authorization', 'Bearer ' || private.supabase_service_key()
    ),
    body := jsonb_build_object(
      'record', to_jsonb(new),
      'event_type', TG_OP
    ),
    timeout_milliseconds := 5000
  );
  RETURN new;
END;
$$;

CREATE TRIGGER on_order_insert
  AFTER INSERT ON public.orders
  FOR EACH ROW EXECUTE FUNCTION public.on_new_order();
```

## Deprecated Endpoint

`POST /v1/projects/{ref}/functions` (create) is **deprecated**. Use `/deploy` instead.
