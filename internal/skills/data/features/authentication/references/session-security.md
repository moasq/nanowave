# Session Security

## Contents
- Token storage
- Token refresh
- Sign out scopes
- Secure logout flow
- Expired session handling
- Account deletion
- Apple credential revocation
- Security rules

## Token Storage

Supabase Swift SDK stores tokens in iOS Keychain by default — no manual Keychain code needed. Custom storage is possible via `AuthLocalStorage` protocol in `SupabaseClientOptions.auth.storage`. Tokens persist across app launches so the user stays logged in.

## Token Refresh

Fully automatic — never manage tokens manually.

```swift
// SDK auto-refreshes before expiry — just observe the event
for await (event, session) in client.auth.authStateChanges {
    case .tokenRefreshed:
        // Automatic — no action needed. SDK already has new tokens.
    case .signedOut:
        // Refresh failed (e.g., refresh token expired) — user must re-authenticate
}

// Access current session (refreshes if needed)
let session = try await client.auth.session
// session.accessToken — current valid JWT
// session.expiresAt — expiration timestamp
```

## Sign Out Scopes

```swift
// Local only — clears tokens on this device, server session stays active
try await client.auth.signOut()  // default scope: .local

// Global — revokes ALL sessions across ALL devices
try await client.auth.signOut(scope: .global)

// Others — revokes all OTHER device sessions, keeps current
try await client.auth.signOut(scope: .others)
```

## Secure Logout Flow

Always follow this order:

1. Call `client.auth.signOut(scope:)` — revokes server session + clears local Keychain tokens
2. Reset `AuthService` state: `isAuthenticated = false`, `currentUserID = nil`, `currentUser = nil`
3. UI reacts automatically via `@Observable` — AuthGuardView switches to login
4. If server call fails: still clear local state (graceful degradation)

```swift
func signOut() async {
    do {
        try await client.auth.signOut(scope: .global)
    } catch {
        // Server unreachable — still clear locally
    }
    isAuthenticated = false
    currentUserID = nil
    currentUser = nil
}
```

## Expired Session Handling

```swift
// When any API call fails with session error:
// 1. SDK attempts automatic refresh
// 2. If refresh fails → .signedOut event emitted
// 3. AuthService observes event → sets isAuthenticated = false
// 4. UI shows login screen

// AuthService session observer (runs on app launch):
func observeAuthState() async {
    for await (event, session) in client.auth.authStateChanges {
        switch event {
        case .signedIn:
            isAuthenticated = true
            currentUserID = session?.user.id
        case .signedOut:
            isAuthenticated = false
            currentUserID = nil
            currentUser = nil
        case .tokenRefreshed:
            break  // Automatic — SDK handles it
        case .userUpdated:
            // Re-fetch profile if needed
            break
        default:
            break
        }
    }
}
```

## Account Deletion

- Client calls a Supabase Edge Function (`delete-user`) with bearer token
- Edge Function uses admin API to delete from `auth.users`
- `ON DELETE CASCADE` on profiles table cleans up all user data
- After deletion: clear local state same as signOut
- Apple App Store requires account deletion option if app has sign-up

```swift
func deleteAccount() async throws {
    try await client.functions.invoke("delete-user")
    // Local cleanup
    isAuthenticated = false
    currentUserID = nil
    currentUser = nil
}
```

## Apple Credential Revocation

When user deletes account, the server revokes the Apple credential. Check credential state on app launch for Apple Sign In users:

```swift
// Check Apple credential validity on launch (only for Apple Sign In users)
let provider = ASAuthorizationAppleIDProvider()
let state = try await provider.credentialState(forUserID: appleUserID)
if state == .revoked || state == .notFound {
    await signOut()
}
```

## Security Rules

- NEVER store tokens in UserDefaults or @AppStorage — Keychain only (SDK does this)
- NEVER log access tokens or include them in analytics
- NEVER pass tokens between views — use AuthService as single source of truth
- ALWAYS use `.global` scope for logout when security matters (e.g., password change, account compromise)
- ALWAYS implement session restore on app launch — don't force re-login
- ALWAYS handle the `.signedOut` event — it fires on token refresh failure too
