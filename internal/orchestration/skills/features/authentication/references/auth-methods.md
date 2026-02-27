# Auth Methods

## Contents
- Auth method selection rules
- Email/password (default)
- Anonymous/guest (default)
- Apple Sign In (explicit only)
- Google Sign In (explicit only)
- Password reset

## Auth Method Selection Rules

- User says nothing about auth method → email/password + anonymous guest
- User says "Apple Sign In" or "Sign in with Apple" → Apple Sign In only
- User says "Google Sign In" → Google Sign In (+ GoogleSignIn package dependency)
- User says "social login" → Apple Sign In (Apple is the native social login on iOS; Google requires manual Cloud Console setup)
- User says "guest mode" or "browse without account" → anonymous only
- Multiple methods can combine: email + Apple + anonymous

## Email/Password (Default)

```swift
// Sign up — pass username in metadata for profile trigger
let response = try await client.auth.signUp(
    email: email,
    password: password,
    data: ["username": .string(username)]
)
// response.session is nil if email confirmation required

// Sign in
let session = try await client.auth.signIn(
    email: email,
    password: password
)

// Password reset — sends magic link email
try await client.auth.resetPasswordForEmail(email)
```

## Anonymous/Guest (Default Alongside Email)

```swift
// Create anonymous session — user can browse and upgrade later
let session = try await client.auth.signInAnonymously()

// Check if current user is anonymous
let isAnonymous = client.auth.currentUser?.isAnonymous ?? false

// Upgrade anonymous to permanent account (link email)
try await client.auth.update(user: UserAttributes(email: email, password: password))
```

## Apple Sign In (Only When User Explicitly Requests)

```swift
import AuthenticationServices

// REQUIRES: com.apple.developer.applesignin entitlement via XcodeGen MCP
// Apple provider is auto-configured by nanowave pipeline

SignInWithAppleButton { request in
    request.requestedScopes = [.email, .fullName]
} onCompletion: { result in
    Task {
        guard let credential = try result.get().credential as? ASAuthorizationAppleIDCredential,
              let idToken = credential.identityToken
                  .flatMap({ String(data: $0, encoding: .utf8) })
        else { return }
        try await client.auth.signInWithIdToken(
            credentials: .init(provider: .apple, idToken: idToken)
        )
    }
}

// Check Apple credential validity on app launch
let provider = ASAuthorizationAppleIDProvider()
let state = try await provider.credentialState(forUserID: storedAppleUserID)
if state == .revoked || state == .notFound {
    await authService.signOut()
}
```

## Google Sign In (Only When User Explicitly Requests)

```swift
// REQUIRES: GoogleSignIn package dependency
// Google provider is auto-configured by nanowave pipeline
import GoogleSignIn

guard let rootVC = UIApplication.shared.connectedScenes
    .compactMap({ $0 as? UIWindowScene }).first?.windows.first?.rootViewController
else { return }

let result = try await GIDSignIn.sharedInstance.signIn(withPresenting: rootVC)
guard let idToken = result.user.idToken?.tokenString else { return }
try await client.auth.signInWithIdToken(
    credentials: .init(provider: .google, idToken: idToken)
)
```

## Password Reset

```swift
// Included automatically when email auth is used
// Sends password reset email with magic link
try await client.auth.resetPasswordForEmail(email)

// Handle deep link callback (in app's URL handler)
// Supabase SDK handles the token exchange automatically
```
