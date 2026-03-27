---
name: "authentication"
description: "Authentication architecture with email/password, Apple Sign In, Google Sign In, and anonymous guest modes. Covers AuthService as @Observable @MainActor, AuthGuardView gate pattern, optional auth flow, Supabase delegation, and session lifecycle management. Use when implementing user authentication, login flows, or session management in an iOS app."
---

# Authentication Architecture

## Auth Method Selection

Default is email/password + anonymous guest. Only add other methods when the user explicitly asks:

| User Says | Auth Methods |
|-----------|-------------|
| *(nothing specified)* | email/password + anonymous guest |
| "Apple Sign In" / "Sign in with Apple" | Apple Sign In |
| "Google Sign In" | Google Sign In (requires GoogleSignIn package) |
| "social login" | Apple + Google |
| "guest mode" / "browse without account" | anonymous only |

Methods combine — a user can request email + Apple + anonymous together.

## Service Architecture

`AuthService` is `@Observable @MainActor` — it owns all auth state and lives in `Services/Auth/`, never inside `Features/`.

```swift
// ViewModels consume AuthService via init injection
@Observable @MainActor
class ProfileViewModel {
    private let authService: AuthService
    init(authService: AuthService) { self.authService = authService }
}

// Views access AuthService through @Environment
@Environment(AuthService.self) private var authService
```

When the Supabase skill is also loaded, `AuthService` delegates to `SupabaseService.shared.client.auth` for all backend calls. Without a backend skill, `AuthService` uses a placeholder implementation.

## Auth Modes

**Gate mode (default):** `AuthGuardView` sits at the app root. Users must authenticate before accessing any content.

**Optional mode** (when user says "auth is optional" or "browse without login"): No root gate. Individual features check `authService.isAuthenticated` and prompt login when needed.

## Key Rules

- Services live in `Services/Auth/` — never inside `Features/`
- ViewModels consume `AuthService` via init injection, never call auth APIs directly
- Never manage tokens manually — the backend SDK handles token storage and refresh
- Always restore session on app launch
- Always handle sign-out gracefully — clear local state even if the server call fails

## References

- [Auth Methods](references/auth-methods.md) — email, guest, Apple, Google patterns with Swift code
- [Auth Architecture](references/auth-architecture.md) — AuthService, directory structure, environment injection
- [Auth Guard](references/auth-guard.md) — gate mode vs optional mode patterns
- [Session Security](references/session-security.md) — token lifecycle, sign out, account deletion
