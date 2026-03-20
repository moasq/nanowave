---
description: "SwiftUI navigation implementation patterns and code examples"
---
# SwiftUI Navigation Reference

Comprehensive guide to NavigationStack, TabView, sheets, fullScreenCover, and routing patterns.

## Pattern Selection Guide

| Pattern | When to Use |
|---------|-------------|
| `NavigationStack` | Hierarchical drill-down (list → detail → edit) |
| `TabView` with `Tab` API | 3+ distinct top-level peer sections |
| `.sheet(item:)` | Creation forms, secondary actions, settings |
| `.fullScreenCover` | Immersive experiences (media player, onboarding) |
| `NavigationStack` + `.sheet` | Most MVPs with 2-4 features |


## TabView

Use when the app has 3+ distinct, peer-level sections:

```swift
TabView {
    Tab("Home", systemImage: "house") {
        HomeView()
    }
    Tab("Search", systemImage: "magnifyingglass") {
        SearchView()
    }
    Tab("Profile", systemImage: "person") {
        ProfileView()
    }
}
```


## Full Screen Cover

```swift
@State private var showOnboarding = false

.fullScreenCover(isPresented: $showOnboarding) {
    OnboardingView()
}
```


## Alert with Actions

```swift
.alert("Delete Item?", isPresented: $showAlert) {
    Button("Delete", role: .destructive) { deleteItem() }
    Button("Cancel", role: .cancel) { }
} message: {
    Text("This action cannot be undone.")
}
```


## Type-Safe Routing

Always use `navigationDestination(for:)` for type-safe routing:

```swift
.navigationDestination(for: Note.self) { note in
    NoteDetailView(note: note)
}
.navigationDestination(for: Category.self) { category in
    CategoryView(category: category)
}
```
