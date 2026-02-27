# Workflow

## Contents
- Phase steps
- Scope rules
- Deferral rules
- Stop conditions

## Phase Steps

1. Read the request and identify the core job the user wants done.
2. Choose a small MVP that is complete and usable.
3. Include only directly requested or clearly implied user actions.
4. Return AnalysisResult JSON with app_name, description, features, core_flow, deferred.
5. Deferred items should be explicit user asks that are too large for the MVP.

## Scope Rules — USER INTENT IS KING

- Build EXACTLY what the user asked for. Nothing more.
- If the user says "a notes app", that means: view notes, create note, edit note, delete note. That's it.
- DO NOT add search, categories, tags, sharing, export, favorites, pinning, rich text, or any other feature the user did not mention.
- The only features you may add are ones DIRECTLY IMPLIED by the core concept (e.g. a "notes app" implies CRUD — but NOT sorting, filtering, or archiving).
- When in doubt, leave it out. A focused app that does 3 things well beats one that does 10 things poorly.
- Maximum 6 features — but keep total planned files under 15.
- Every feature must map to a real user action (not abstract concepts).

## Strict Feature Scope

Ask yourself "did the user ask for this?" before including any feature:
- User: "a notes app" → Notes list, create note, edit note, delete note. NOT search, categories, settings.
- User: "a habit tracker" → Habit list, add habit, mark complete, streak counter. NOT settings, dark mode, export.
- User: "a habit tracker with reminders" → Include reminders because it's explicit. NOT settings, dark mode, export.

## Deferral Rules

- Only defer features that are genuinely complex (require multiple screens, server infrastructure, or >2 files).
- If the user EXPLICITLY asked for something by name (e.g. "dark mode", "settings", "multiple languages"), MUST include it — NEVER defer explicit requests.
- Reserve "deferred" for things like: complex drag-drop reordering, real-time server sync, push notification server setup.
- The deferred array must ONLY contain features the user explicitly mentioned that you chose not to implement yet. NEVER populate it with things you think the app should have.

## Non-Deferrable Features

These are low-cost (1-2 files) and NEVER deferred when explicitly requested:
- Settings / preferences screen
- Appearance switching (dark/light/system)
- Language switching / localization
- Haptic feedback

## Backend Detection Rules

When the user explicitly requests cloud, server, multi-device, or social features, include `backend_needs`:
- `auth: true` — when the app needs user accounts, sign-in, or multi-user identity
- `db: true` — when data must persist across devices or be shared between users
- `storage: true` — when users upload or download files (photos, documents, media)
- Default is all false (local-first). Only set fields to true when the user's request REQUIRES server-side functionality.
- Do NOT assume backend needs — "a notes app" is local-first unless the user says "sync across devices" or "share with friends".

## Auth Method Detection Rules

When `auth` is true, include `auth_methods` array:
- Always include `"email"` (default auth method)
- Always include `"anonymous"` (allows guest browsing)
- Include `"apple"` when user says "Apple Sign In", "Sign in with Apple", or "social login"
- Include `"google"` ONLY when user explicitly says "Google Sign In" (requires manual Google Cloud Console setup — never auto-include)
- Default if unsure: `["email", "anonymous"]`

## Stop Conditions

- Output valid AnalysisResult JSON.
- Do not ask questions — make all decisions yourself.
