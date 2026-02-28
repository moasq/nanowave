# Output Format

Return valid JSON for AnalysisResult. Do not ask questions. Do not output markdown.

```json
{
  "app_name": "string",
  "description": "string",
  "features": [{"name": "string", "description": "string"}],
  "core_flow": "string",
  "deferred": ["string"],
  "backend_needs": {
    "auth": false,
    "auth_methods": ["email", "anonymous"],
    "db": false,
    "storage": false,
    "realtime": false,
    "monetization": false,
    "monetization_type": "subscription"
  }
}
```

## Rules

- `backend_needs` — include only when the app requires cloud/backend features OR monetization.
- `auth_methods` — REQUIRED when `auth` is true. Array of strings from: `"email"`, `"apple"`, `"google"`, `"anonymous"`.
  - Always include `"email"` and `"anonymous"` as baseline.
  - Include `"apple"` when user mentions "Apple", "Sign in with Apple", or "social login".
  - Include `"google"` ONLY when user explicitly says "Google Sign In".
  - Example: user says "sign in with email or Apple" → `["email", "apple", "anonymous"]`
- `realtime` — set to `true` when the app needs live updates (chat, collaborative editing, live feeds, presence). Enables Supabase Realtime publication on all tables.
- `monetization` — set to `true` when the app needs in-app purchases, subscriptions, paywalls, premium tiers, or credit-based usage. Triggers RevenueCat integration.
- `monetization_type` — REQUIRED when `monetization` is true. One of:
  - `"subscription"` — recurring subscription (monthly, yearly, etc.)
  - `"consumable"` — one-time credit packs
  - `"hybrid"` — both subscriptions and consumables
  - Set based on user intent: "paywall", "premium", "pro" → subscription; "credits", "tokens", "packs" → consumable; both → hybrid
