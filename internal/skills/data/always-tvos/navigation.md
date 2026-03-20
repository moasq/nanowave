---
name: "navigation"
description: "tvOS navigation: top TabView tabs, focus-based drill-down, full-screen presentations. Use when working on tvOS navigation patterns, tab bars, or focus-driven transitions. Triggers: TabView, NavigationStack, focus, tab bar, navigation."
---
# Navigation Patterns (tvOS)

## Pattern Selection Guide

| Pattern | When to Use |
|---------|-------------|
| `TabView` (top tabs) | Main app sections (2-7 tabs at top of screen) |
| `NavigationStack` | Hierarchical drill-down within a tab |
| `.fullScreenCover` | Immersive content (player, detail) |
| `.sheet` | Secondary input, settings |
| `.alert` / `.confirmationDialog` | Confirmation, destructive actions |

## TabView — Top Tab Bar (Primary Pattern)
tvOS shows tabs at the TOP of the screen, not bottom:

```swift
TabView {
    Tab("Home", systemImage: "house") {
        HomeView()
    }
    Tab("Browse", systemImage: "square.grid.2x2") {
        BrowseView()
    }
    Tab("Search", systemImage: "magnifyingglass") {
        SearchView()
    }
    Tab("Settings", systemImage: "gear") {
        SettingsView()
    }
}
```

## NavigationStack (Within a Tab)
```swift
NavigationStack {
    ScrollView {
        LazyVStack(alignment: .leading, spacing: 60) {
            ShelfSection(title: "Continue Watching", items: continueWatching)
            ShelfSection(title: "Recommended", items: recommended)
        }
    }
    .navigationDestination(for: Item.self) { item in
        ItemDetailView(item: item)
    }
    .navigationTitle("Home")
}
```

## Full-Screen Cover (Media Playback)
```swift
@State private var playingItem: Item?

.fullScreenCover(item: $playingItem) { item in
    PlayerView(item: item)
}
```

## Type-Safe Routing
```swift
enum Route: Hashable {
    case detail(Item)
    case category(Category)
    case settings
}

NavigationStack {
    ContentView()
        .navigationDestination(for: Route.self) { route in
            switch route {
            case .detail(let item):
                ItemDetailView(item: item)
            case .category(let cat):
                CategoryView(category: cat)
            case .settings:
                SettingsView()
            }
        }
}
```

## Focus-Driven Navigation
The Siri Remote moves focus between elements — no touch events:

```swift
@FocusState private var focusedItem: Item.ID?

ForEach(items) { item in
    Button { select(item) } label: {
        ItemCard(item: item)
    }
    .focused($focusedItem, equals: item.id)
}
.onChange(of: focusedItem) { _, newValue in
    // Update preview/hero when focus changes
    if let id = newValue {
        selectedPreview = items.first(where: { $0.id == id })
    }
}
```

## NOT Available on tvOS
- No bottom tab bar — tabs are ALWAYS at the top
- No swipe gestures — focus is moved with Siri Remote directional pad
- No `.popover` — use `.sheet` or `.fullScreenCover`
- No `NavigationSplitView` sidebar — use TabView with sections
- No `.presentationDetents` — sheets have fixed presentation
- No drag-and-drop
- No pull-to-refresh

## Navigation Rules
1. Use `TabView` with top tabs for main app sections
2. Use `NavigationStack` for drill-down within a tab
3. All navigable elements must be focusable — test with Siri Remote
4. Use `.fullScreenCover` for immersive content like media playback
5. Keep navigation depth shallow — 2-3 levels max for TV experience
6. Swiping up on the Siri Remote reveals the top tab bar — don't override this
