# Auth Architecture

## Contents
- Directory structure
- AuthService pattern
- ViewModel consumption
- Environment injection

## Directory Structure

```
Services/
  Auth/
    AuthService.swift        → @Observable @MainActor, owns auth state
App/
  {AppName}App.swift         → Injects AuthService into environment
Features/
  Auth/
    AuthView.swift           → Login/signup UI
    AuthViewModel.swift      → Consumes AuthService
  Common/
    AuthGuardView.swift      → Root gate view (if gate mode)
Shared/
  Loadable.swift             → Loadable<T> enum (shared async state type)
```

Services live OUTSIDE `Features/` — they handle business logic and external integrations, not presentation.

## AuthService Pattern

```swift
@Observable
@MainActor
final class AuthService {
    // State — Loadable tracks session restore lifecycle
    // .notInitiated = app just launched, no restore attempted
    // .loading = restoring session
    // .success(profile) = restored, profile is the logged-in user (nil = not authenticated)
    // .failure(error) = restore failed
    var session: Loadable<UserProfile?> = .notInitiated

    // Computed conveniences
    var currentUser: UserProfile? {
        if case .success(let profile) = session { return profile }
        return nil
    }

    var isAuthenticated: Bool { currentUser != nil }

    // Auth methods (backend-specific implementation provided by Supabase or other skill)
    func signUp(email: String, password: String, username: String) async throws
    func signIn(email: String, password: String) async throws
    func signInAnonymously() async throws
    func signInWithApple(idToken: String) async throws
    func signOut() async  // Always succeeds — clears local state even if server fails
    func resetPassword(email: String) async throws
    func deleteAccount() async throws

    // Session lifecycle
    func restoreSession() async  // Called on app launch — sets .loading → .success(profile) or .success(nil)
    func observeAuthState() async  // Listens to authStateChanges stream

    // Computed
    var isAnonymous: Bool  // true if guest user
}
```

## ViewModel Consumption

ViewModels receive `AuthService` via init — never create their own instance.

```swift
@Observable
@MainActor
class ProfileViewModel {
    private let authService: AuthService

    init(authService: AuthService) {
        self.authService = authService
    }

    var isLoggedIn: Bool { authService.isAuthenticated }
}
```

## Environment Injection

The app entry point creates `AuthService` and injects it into the environment.

```swift
@main
struct MyApp: App {
    @State private var authService = AuthService()

    var body: some Scene {
        WindowGroup {
            AuthGuardView()
                .environment(authService)
        }
    }
}
```
