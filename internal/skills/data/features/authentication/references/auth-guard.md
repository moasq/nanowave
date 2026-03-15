# Auth Guard

## Contents
- Gate mode (default)
- Optional mode
- Session restoration

## Gate Mode (Default)

Root view that switches between login and main content based on auth state. Use this when the entire app requires authentication.

```swift
struct AuthGuardView: View {
    @Environment(AuthService.self) var authService

    var body: some View {
        Group {
            switch authService.session {
            case .notInitiated, .loading:
                ProgressView()
            case .success(let user) where user != nil:
                MainTabView()
            case .success:
                AuthView()
            case .failure:
                ContentUnavailableView {
                    Label("Session Error", systemImage: "exclamationmark.triangle")
                } description: {
                    Text("Could not restore your session.")
                } actions: {
                    Button("Retry") { Task { await authService.restoreSession() } }
                }
            }
        }
        .task { await authService.restoreSession() }
    }
}
```

## Optional Mode

Use when the user says "auth is optional", "browse without login", or "some features require login". No gate at root — user browses freely. Individual features check auth and show login prompt when needed.

```swift
// No gate at root — user browses freely
// Individual features check auth and show login prompt when needed
struct PostDetailView: View {
    @Environment(AuthService.self) var authService
    @State private var showAuth = false

    var body: some View {
        // Content always visible
        ScrollView { ... }
            .toolbar {
                if authService.isAuthenticated {
                    Button("Comment") { ... }
                } else {
                    Button("Sign in to comment") { showAuth = true }
                }
            }
            .sheet(isPresented: $showAuth) { AuthView() }
    }
}
```

## Session Restoration

Always attempt session restore on app launch:
- **Gate mode**: shows `ProgressView` while session is `.notInitiated` or `.loading`, then switches to authenticated or login view based on `.success` result
- **Optional mode**: restore silently in background — user can browse immediately
