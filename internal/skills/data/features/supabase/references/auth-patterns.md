# Auth Patterns

## Contents
- Profile table setup (required for auth)
- Email/password auth
- Sign in with Apple (ASAuthorizationController + signInWithIdToken)
- Auth state observation
- Sign out
- XcodeGen entitlement setup

## Profile Table Setup (Backend-First)

BEFORE writing any auth Swift code, create the profiles table and its policies:

```sql
-- Profiles table linked to auth.users
CREATE TABLE IF NOT EXISTS public.profiles (
  id UUID PRIMARY KEY REFERENCES auth.users(id) ON DELETE CASCADE,
  username TEXT UNIQUE NOT NULL,
  avatar_url TEXT,
  created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
  updated_at TIMESTAMPTZ DEFAULT now() NOT NULL
);

ALTER TABLE public.profiles ENABLE ROW LEVEL SECURITY;

-- Anyone can read profiles
CREATE POLICY "profiles are viewable by everyone"
  ON public.profiles FOR SELECT USING (true);

-- Users can insert their own profile
CREATE POLICY "users can insert own profile"
  ON public.profiles FOR INSERT
  WITH CHECK (auth.uid() = id);

-- Users can update only their own profile
CREATE POLICY "users can update own profile"
  ON public.profiles FOR UPDATE
  USING (auth.uid() = id)
  WITH CHECK (auth.uid() = id);
```

Add a trigger to auto-create a profile when a user signs up:

```sql
-- Auto-create profile on signup
CREATE OR REPLACE FUNCTION public.handle_new_user()
RETURNS TRIGGER AS $$
BEGIN
  INSERT INTO public.profiles (id, username)
  VALUES (
    NEW.id,
    COALESCE(NEW.raw_user_meta_data->>'username', 'user_' || LEFT(NEW.id::text, 8))
  );
  RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE TRIGGER on_auth_user_created
  AFTER INSERT ON auth.users
  FOR EACH ROW EXECUTE FUNCTION public.handle_new_user();
```

This trigger eliminates the need for client-side profile creation after signup.

## Email/Password Auth

```swift
// Sign up — pass username in metadata
// Returns AuthResponse with optional session (nil if email confirmation required)
let response = try await SupabaseService.shared.client.auth.signUp(
    email: email,
    password: password,
    data: ["username": .string(username)]
)
// response.session is non-nil if auto-confirm is enabled
// response.user contains user data either way

// Sign in — returns Session directly
let session = try await SupabaseService.shared.client.auth.signIn(
    email: email,
    password: password
)
```

The `data` parameter in `signUp` sets `raw_user_meta_data` which the trigger reads.

## Sign in with Apple

Split responsibility: Apple's ASAuthorizationController handles native UI and token generation.
Supabase `signInWithIdToken` handles user creation, session management, and JWT refresh.

```swift
import AuthenticationServices
import Supabase

SignInWithAppleButton { request in
    request.requestedScopes = [.email, .fullName]
} onCompletion: { result in
    Task {
        guard let credential = try result.get().credential as? ASAuthorizationAppleIDCredential,
              let idToken = credential.identityToken.flatMap({ String(data: $0, encoding: .utf8) })
        else { return }
        try await SupabaseService.shared.client.auth.signInWithIdToken(
            credentials: .init(provider: .apple, idToken: idToken)
        )
    }
}
```

**XcodeGen entitlement**: The builder MUST call `mcp__xcodegen__add_entitlement` with key `com.apple.developer.applesignin` and value `["Default"]` to enable Sign in with Apple capability.

## Auth State Observation

Use `authStateChanges` AsyncStream to reactively update UI.

```swift
for await (event, session) in SupabaseService.shared.client.auth.authStateChanges {
    switch event {
    case .signedIn:
        print("User signed in: \(session?.user.id ?? UUID())")
    case .signedOut:
        print("User signed out")
    case .tokenRefreshed:
        print("Token refreshed — automatic, no action needed")
    case .userUpdated:
        print("User data updated")
    default:
        break
    }
}
```

## Sign Out

```swift
try await SupabaseService.shared.client.auth.signOut()
```

Auth architecture (AuthService, AuthGuardView, gate/optional modes) is defined in the `authentication` skill. This file covers only the Supabase API calls.
