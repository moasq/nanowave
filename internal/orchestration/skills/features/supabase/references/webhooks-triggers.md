# Database Webhooks & Triggers

## Contents
- Creating database webhooks via `supabase_functions.http_request()`
- Custom trigger functions with `pg_net`
- Secure patterns using Supabase Vault
- Payload format

## Built-in Webhook Function

Supabase provides `supabase_functions.http_request()` — a trigger function that sends async HTTP requests when rows change.

### Simple Webhook (POST on INSERT)

```sql
CREATE TRIGGER my_webhook AFTER INSERT
ON public.orders FOR EACH ROW
EXECUTE FUNCTION supabase_functions.http_request(
  'https://example.com/webhook',
  'POST',
  '{"Content-Type":"application/json"}',
  '{}',
  '1000'
);
```

### Parameters (Positional via TG_ARGV)

| Position | Name | Type | Default | Description |
|----------|------|------|---------|-------------|
| 0 | url | text | required | HTTP endpoint |
| 1 | method | text | required | `'GET'` or `'POST'` only |
| 2 | headers | jsonb | `'{"Content-Type":"application/json"}'` | Request headers |
| 3 | params | jsonb | `'{}'` | Query parameters |
| 4 | timeout_ms | integer | `1000` | Timeout in milliseconds |

### POST Payload (Auto-Generated)

```json
{
  "old_record": { ... },
  "record": { ... },
  "type": "INSERT|UPDATE|DELETE",
  "table": "orders",
  "schema": "public"
}
```

### Multi-Event Webhook

Separate triggers are needed for each event type:

```sql
CREATE TRIGGER orders_insert_webhook AFTER INSERT
ON public.orders FOR EACH ROW
EXECUTE FUNCTION supabase_functions.http_request(
  'https://example.com/webhook', 'POST',
  '{"Content-Type":"application/json","Authorization":"Bearer KEY"}',
  '{}', '5000'
);

CREATE TRIGGER orders_update_webhook AFTER UPDATE
ON public.orders FOR EACH ROW
EXECUTE FUNCTION supabase_functions.http_request(
  'https://example.com/webhook', 'POST',
  '{"Content-Type":"application/json","Authorization":"Bearer KEY"}',
  '{}', '5000'
);
```

## Custom Trigger with pg_net (Recommended for Edge Functions)

For custom payloads or dynamic headers, use `pg_net` directly:

```sql
CREATE EXTENSION IF NOT EXISTS pg_net SCHEMA extensions;

CREATE OR REPLACE FUNCTION public.notify_edge_function()
RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER
SET search_path = public AS $$
DECLARE
  payload jsonb;
BEGIN
  payload := jsonb_build_object(
    'type', TG_OP,
    'table', TG_TABLE_NAME,
    'schema', TG_TABLE_SCHEMA
  );

  IF TG_OP = 'DELETE' THEN
    payload := payload || jsonb_build_object('record', null, 'old_record', to_jsonb(old));
  ELSIF TG_OP = 'UPDATE' THEN
    payload := payload || jsonb_build_object('record', to_jsonb(new), 'old_record', to_jsonb(old));
  ELSIF TG_OP = 'INSERT' THEN
    payload := payload || jsonb_build_object('record', to_jsonb(new), 'old_record', null);
  END IF;

  PERFORM net.http_post(
    url := 'https://PROJECT_REF.supabase.co/functions/v1/my-function',
    headers := jsonb_build_object(
      'Content-Type', 'application/json',
      'Authorization', 'Bearer ' || private.supabase_service_key()
    ),
    body := payload,
    timeout_milliseconds := 5000
  );

  IF TG_OP = 'DELETE' THEN RETURN old; END IF;
  RETURN new;
END;
$$;

CREATE TRIGGER on_order_change
  AFTER INSERT OR UPDATE OR DELETE ON public.orders
  FOR EACH ROW EXECUTE FUNCTION public.notify_edge_function();
```

## Secure Key Storage with Vault

Never hardcode API keys in trigger functions. Use Supabase Vault:

```sql
-- Store key in Vault (run once via Dashboard or migration)
SELECT vault.create_secret('your-service-role-key', 'supabase_service_key');

-- Create helper function to retrieve it
CREATE OR REPLACE FUNCTION private.supabase_service_key()
RETURNS text LANGUAGE plpgsql SECURITY DEFINER AS $$
DECLARE secret_value text;
BEGIN
  SELECT decrypted_secret INTO secret_value
  FROM vault.decrypted_secrets WHERE name = 'supabase_service_key';
  RETURN secret_value;
END;
$$;
```

## pg_net Function Signatures

```sql
-- Async POST
net.http_post(
  url text,
  body jsonb DEFAULT '{}'::jsonb,
  params jsonb DEFAULT '{}'::jsonb,
  headers jsonb DEFAULT '{"Content-Type":"application/json"}'::jsonb,
  timeout_milliseconds int DEFAULT 1000
) RETURNS bigint  -- request ID

-- Async GET
net.http_get(
  url text,
  params jsonb DEFAULT '{}'::jsonb,
  headers jsonb DEFAULT '{}'::jsonb,
  timeout_milliseconds int DEFAULT 1000
) RETURNS bigint

-- Async DELETE
net.http_delete(
  url text,
  params jsonb DEFAULT '{}'::jsonb,
  headers jsonb DEFAULT '{}'::jsonb,
  timeout_milliseconds int DEFAULT 2000
) RETURNS bigint
```

Requests execute **after the transaction commits**. Responses are stored in `net._http_response` for 6 hours.

## Key Gotchas

1. `supabase_functions.http_request()` is a **trigger function only** — cannot be called via `SELECT` or `PERFORM`
2. For custom logic inside PL/pgSQL, use `net.http_post()` directly
3. Requests only fire after transaction commit — rollback = no request
4. `pg_net` rate limit: ~200 requests/second
5. Remove a webhook: `DROP TRIGGER my_webhook ON public.orders;`
